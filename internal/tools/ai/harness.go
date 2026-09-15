package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"caramel/internal/prompts"
)

// GenerationItem representa um item a ser sintetizado e gerado pelo Harness
type GenerationItem struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Prompt    string `json:"prompt"`
	Status    string `json:"status"` // pending, generating, done, error
	ImagePath string `json:"image_path,omitempty"`
	Format    string `json:"format,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HarnessConfig contém todas as configurações do pipeline em lote
type HarnessConfig struct {
	Items       []string
	Theme       string
	Count       int
	Style       string
	CustomStyle string
	OutputDir   string
	MaxWorkers  int
	TextModel   string
	ImageModel  string
	Aspect      string // Proporção das imagens geradas (ex: "1:1", "16:9"; vazio = 1:1)
	Verbose     bool
}

// HarnessProgressEvent transporta informações em tempo real do progresso
type HarnessProgressEvent struct {
	Item        GenerationItem
	Total       int
	Completed   int
	Success     int
	Failed      int
	CurrentStep string // "synthesizing", "generating", "saved", "done"
}

// HarnessProgressFunc é o callback para atualização da interface CLI / TUI
type HarnessProgressFunc func(event HarnessProgressEvent)

// slugRegex remove tudo que não seja letra (Unicode, preservando acentos como ç/ã/é),
// número ou underscore — essencial para as legendas das fichas manterem a grafia correta
var slugRegex = regexp.MustCompile(`[^\p{L}\p{N}_]`)

// SanitizeSlug transforma um nome em slug seguro para arquivo
func SanitizeSlug(index int, name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.ReplaceAll(lower, " ", "_")
	slug := slugRegex.ReplaceAllString(lower, "")
	if slug == "" {
		slug = "item"
	}
	return fmt.Sprintf("%02d_%s", index, slug)
}

// SynthesizePrompts utiliza a LLM de texto para gerar prompts consistentes e estruturados
func SynthesizePrompts(cfg HarnessConfig, client *Client) ([]GenerationItem, error) {
	return SynthesizePromptsContext(context.Background(), cfg, client)
}

// SynthesizePromptsContext is the cancelable variant of SynthesizePrompts.
func SynthesizePromptsContext(ctx context.Context, cfg HarnessConfig, client *Client) ([]GenerationItem, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	effectiveStyle := cfg.Style
	if cfg.CustomStyle != "" {
		effectiveStyle = cfg.CustomStyle
	}
	if effectiveStyle == "" {
		effectiveStyle = "clipart"
	}

	synthesizerPrompt := prompts.GetPromptSynthesizerPrompt(effectiveStyle)

	var inputText string
	if len(cfg.Items) > 0 {
		inputText = "LIST OF ITEMS TO GENERATE:\n"
		for i, it := range cfg.Items {
			inputText += fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(it))
		}
	} else if cfg.Theme != "" {
		count := cfg.Count
		if count <= 0 {
			count = 10
		}
		inputText = fmt.Sprintf("THEME: %s\nPlease select the %d most iconic/representative items for this theme and generate their visual prompts.", cfg.Theme, count)
	} else {
		return nil, fmt.Errorf("nenhum item ou tema informado para geração de prompts")
	}

	model := cfg.TextModel
	if model == "" {
		model = DefaultTextModel
	}

	var responseJSON string
	err := retryWithBackoffContext(ctx, 3, func() error {
		var analyzeErr error
		responseJSON, analyzeErr = client.AnalyzeRoutineContext(ctx, inputText, synthesizerPrompt, model)
		return analyzeErr
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao sintetizar prompts com a IA: %w", err)
	}

	// Remove blocos de markdown caso venha ```json ... ```
	cleanedJSON := strings.TrimSpace(responseJSON)
	if idxStart := strings.Index(cleanedJSON, "["); idxStart != -1 {
		if idxEnd := strings.LastIndex(cleanedJSON, "]"); idxEnd != -1 && idxEnd > idxStart {
			cleanedJSON = cleanedJSON[idxStart : idxEnd+1]
		}
	}

	var items []GenerationItem
	if err := json.Unmarshal([]byte(cleanedJSON), &items); err != nil {
		return nil, fmt.Errorf("falha ao interpretar lista JSON gerada pela IA: %w", err)
	}

	// Normaliza índices e slugs
	for i := range items {
		items[i].Index = i + 1
		if items[i].Slug == "" {
			items[i].Slug = SanitizeSlug(items[i].Index, items[i].Name)
		}
		items[i].Status = "pending"
	}

	return items, nil
}

// CalculateConcurrencyDecision calcula o número ideal de workers e delay baseado em N
func CalculateConcurrencyDecision(total int, userWorkers int) (workers int, dispatchDelay time.Duration) {
	if userWorkers > 0 {
		return userWorkers, 100 * time.Millisecond
	}

	if total <= 3 {
		// Modo Direct Burst: concorrência total imediata
		return total, 0
	} else if total <= 10 {
		// Modo Lotes Pequenos: pool de 4 workers com delay suave
		return 4, 150 * time.Millisecond
	}

	// Modo Adaptativo para lotes grandes: pool de 5 workers com delay de espaçamento
	return 5, 300 * time.Millisecond
}

// ExecuteGenerationHarness processa a lista de prompts com concorrência adaptativa e retries
func ExecuteGenerationHarness(items []GenerationItem, cfg HarnessConfig, client *Client, onProgress HarnessProgressFunc) ([]GenerationItem, error) {
	return ExecuteGenerationHarnessContext(context.Background(), items, cfg, client, onProgress)
}

// ExecuteGenerationHarnessContext é a variante cancelável do harness de geração.
// O cancelamento é observado entre itens; a chamada HTTP corrente termina pelo
// timeout normal do cliente quando a API não oferece interrupção imediata.
func ExecuteGenerationHarnessContext(ctx context.Context, items []GenerationItem, cfg HarnessConfig, client *Client, onProgress HarnessProgressFunc) ([]GenerationItem, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return items, err
	}
	if len(items) == 0 {
		return items, nil
	}

	targetDir := cfg.OutputDir
	if targetDir == "" {
		targetDir = fmt.Sprintf("./imagens_geradas_%s", time.Now().Format("20060102_150405"))
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pasta de destino '%s': %w", targetDir, err)
	}

	total := len(items)
	results := make([]GenerationItem, total)
	copy(results, items)
	_, batchErr := ExecuteBatchContext(ctx, total, cfg.MaxWorkers, func(workCtx context.Context, idx int) error {
		item := results[idx]
		var imgBytes []byte
		var ext string
		genErr := retryWithBackoffContext(workCtx, 3, func() error {
			var e error
			imgBytes, ext, e = client.GenerateImageContext(workCtx, item.Prompt, cfg.ImageModel, cfg.Aspect)
			return e
		})
		if genErr != nil {
			results[idx].Status = "error"
			results[idx].Error = genErr.Error()
			return genErr
		}

		if ext == "" {
			ext = "png"
		}
		fileName := fmt.Sprintf("%s.%s", item.Slug, ext)
		filePath := filepath.Join(targetDir, fileName)
		if writeErr := os.WriteFile(filePath, imgBytes, 0644); writeErr != nil {
			results[idx].Status = "error"
			results[idx].Error = fmt.Sprintf("falha ao salvar arquivo: %v", writeErr)
			return writeErr
		}

		results[idx].Status = "done"
		results[idx].ImagePath = filePath
		results[idx].Format = ext
		return nil
	}, func(event BatchProgressEvent) {
		if onProgress == nil {
			return
		}
		if event.State == "started" {
			results[event.Index].Status = "generating"
			onProgress(HarnessProgressEvent{
				Item:        results[event.Index],
				Total:       event.Total,
				Completed:   event.Completed,
				Success:     event.Succeeded,
				Failed:      event.Failed,
				CurrentStep: "generating",
			})
			return
		}
		if event.State == "completed" || event.State == "failed" {
			onProgress(HarnessProgressEvent{
				Item:        results[event.Index],
				Total:       event.Total,
				Completed:   event.Completed,
				Success:     event.Succeeded,
				Failed:      event.Failed,
				CurrentStep: "saved",
			})
			return
		}
		if event.State == "done" {
			onProgress(HarnessProgressEvent{
				Total:       event.Total,
				Completed:   event.Completed,
				Success:     event.Succeeded,
				Failed:      event.Failed,
				CurrentStep: "done",
			})
		}
	})
	if batchErr != nil {
		return results, batchErr
	}
	return results, nil
}
