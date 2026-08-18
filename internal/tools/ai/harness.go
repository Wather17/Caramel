package ai

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

var slugRegex = regexp.MustCompile(`[^a-z0-9_]`)

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

	responseJSON, err := client.AnalyzeRoutine(inputText, synthesizerPrompt, model)
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
		return nil, fmt.Errorf("falha ao interpretar lista JSON gerada pela IA: %w (conteúdo recebido: %s)", err, responseJSON)
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
	workers, dispatchDelay := CalculateConcurrencyDecision(total, cfg.MaxWorkers)

	var mu sync.Mutex
	completedCount := 0
	successCount := 0
	failedCount := 0

	results := make([]GenerationItem, total)
	copy(results, items)

	jobs := make(chan int, total)
	var wg sync.WaitGroup

	// Inicia os workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				item := results[idx]

				mu.Lock()
				results[idx].Status = "generating"
				if onProgress != nil {
					onProgress(HarnessProgressEvent{
						Item:        results[idx],
						Total:       total,
						Completed:   completedCount,
						Success:     successCount,
						Failed:      failedCount,
						CurrentStep: "generating",
					})
				}
				mu.Unlock()

				// Executa com retry com exponential backoff
				maxRetries := 3
				var imgBytes []byte
				var ext string
				var genErr error

				for attempt := 1; attempt <= maxRetries; attempt++ {
					imgBytes, ext, genErr = client.GenerateImage(item.Prompt, cfg.ImageModel)
					if genErr == nil {
						break
					}

					// Se for rate limit ou erro temporário, aplica jitter e backoff
					if attempt < maxRetries {
						backoff := time.Duration(attempt) * 1200 * time.Millisecond
						jitter := time.Duration(rand.Intn(400)) * time.Millisecond
						time.Sleep(backoff + jitter)
					}
				}

				mu.Lock()
				completedCount++
				if genErr != nil {
					failedCount++
					results[idx].Status = "error"
					results[idx].Error = genErr.Error()
				} else {
					if ext == "" {
						ext = "png"
					}
					fileName := fmt.Sprintf("%s.%s", item.Slug, ext)
					filePath := filepath.Join(targetDir, fileName)

					if writeErr := os.WriteFile(filePath, imgBytes, 0644); writeErr != nil {
						failedCount++
						results[idx].Status = "error"
						results[idx].Error = fmt.Sprintf("falha ao salvar arquivo: %v", writeErr)
					} else {
						successCount++
						results[idx].Status = "done"
						results[idx].ImagePath = filePath
						results[idx].Format = ext
					}
				}

				if onProgress != nil {
					onProgress(HarnessProgressEvent{
						Item:        results[idx],
						Total:       total,
						Completed:   completedCount,
						Success:     successCount,
						Failed:      failedCount,
						CurrentStep: "saved",
					})
				}
				mu.Unlock()
			}
		}()
	}

	// Despacha os itens com delay adaptativo
	for i := 0; i < total; i++ {
		jobs <- i
		if dispatchDelay > 0 && i < total-1 {
			time.Sleep(dispatchDelay)
		}
	}
	close(jobs)

	wg.Wait()

	if onProgress != nil {
		onProgress(HarnessProgressEvent{
			Total:       total,
			Completed:   completedCount,
			Success:     successCount,
			Failed:      failedCount,
			CurrentStep: "done",
		})
	}

	return results, nil
}
