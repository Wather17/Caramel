package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/cards"
	"caramel/internal/tools/pdf"
	"caramel/internal/vault"
)

// VaultImageService executa os fluxos usando materiais do acervo global.
type VaultImageService struct {
	Vault        *vault.Vault
	CollectionID string
	Progress     func(ProgressEvent)
}

// Generate gera imagens e importa cada resultado como material do vault.
func (s *VaultImageService) Generate(ctx context.Context, opts ImageOptions) ([]vault.Material, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	if cfg.OpenRouterAPIKey == "" {
		return nil, fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup'")
	}
	if opts.Count <= 0 {
		opts.Count = 10
	}
	if opts.Style == "" {
		opts.Style = "clipart"
	}
	if opts.Aspect == "" {
		opts.Aspect = "1:1"
	}
	if opts.ImageModel == "" {
		opts.ImageModel = orDefault(cfg.ModelImage, ai.DefaultModel)
	}
	if opts.TextModel == "" {
		opts.TextModel = orDefault(cfg.ModelText, ai.DefaultTextModel)
	}
	run, err := s.Vault.StartRun(ctx, s.CollectionID, "generate", map[string]string{"style": opts.Style, "aspect": opts.Aspect, "image_model": opts.ImageModel, "text_model": opts.TextModel}, nil)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "caramel-vault-generate-")
	if err != nil {
		_ = s.Vault.FinishRun(ctx, run.ID, "failed", nil, err)
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	harness := ai.HarnessConfig{Items: opts.Items, Theme: opts.Theme, Count: opts.Count, Style: opts.Style, OutputDir: tempDir, TextModel: opts.TextModel, TextFallbacks: cfg.ModelTextFallbacks, ImageModel: opts.ImageModel, ImageFallbacks: cfg.ModelImageFallbacks, Aspect: opts.Aspect}
	client, err := ai.NewClient(cfg.OpenRouterAPIKey)
	if err != nil {
		_ = s.Vault.FinishRun(ctx, run.ID, "failed", nil, err)
		return nil, err
	}
	attempts := ai.NewAttemptCollector()
	client.AttemptWriter = attempts.Writer()
	s.emit(ProgressEvent{Step: "synthesizing", Message: "Sintetizando prompts..."})
	harness.AttemptWriter = attempts.Writer()
	items, err := ai.SynthesizePromptsContext(ctx, harness, client)
	if err != nil {
		status := "failed"
		if ctx.Err() != nil {
			status = "canceled"
		}
		_ = s.Vault.FinishRunWithAttempts(context.Background(), run.ID, status, nil, err, attempts.Snapshot())
		return nil, err
	}
	results, err := ai.ExecuteGenerationHarnessContext(ctx, items, harness, client, func(event ai.HarnessProgressEvent) {
		message := event.Item.Name
		if event.Item.Error != "" {
			message = event.Item.Error
		}
		s.emit(ProgressEvent{Step: event.CurrentStep, Current: event.Completed, Total: event.Total, Message: message, Path: event.Item.ImagePath})
	})
	// A conclusão do lote pode ser cancelada enquanto resultados já concluídos
	// ainda precisam ser importados e registrados no estado terminal.
	persistCtx := context.Background()
	var materials []vault.Material
	for _, result := range results {
		if result.Status != "done" || result.ImagePath == "" {
			continue
		}
		imported, importErr := s.Vault.ImportFile(persistCtx, result.ImagePath, result.Name, []string{"gerado", opts.Style})
		if importErr != nil {
			_ = s.Vault.FinishRunWithAttempts(persistCtx, run.ID, "failed", materialIDs(materials), importErr, attempts.Snapshot())
			return materials, importErr
		}
		materials = append(materials, imported.Material)
		attempts.AnnotateItem(result.Index, imported.Material.ObjectPath, false)
		if s.CollectionID != "" {
			_ = s.Vault.AddToCollection(persistCtx, s.CollectionID, imported.Material.ID)
		}
	}
	if err != nil {
		status := "failed"
		if ctx.Err() != nil {
			status = "canceled"
		}
		_ = s.Vault.FinishRunWithAttempts(context.Background(), run.ID, status, materialIDs(materials), err, attempts.Snapshot())
		return materials, err
	}
	if err := s.Vault.FinishRunWithAttempts(persistCtx, run.ID, "completed", materialIDs(materials), nil, attempts.Snapshot()); err != nil {
		return materials, err
	}
	s.emit(ProgressEvent{Step: "done", Current: len(materials), Total: len(results), Message: fmt.Sprintf("%d material(is) gerado(s)", len(materials))})
	return materials, nil
}

// Colorize transforma materiais selecionados em novos materiais derivados.
func (s *VaultImageService) Colorize(ctx context.Context, ids []string, opts ImageOptions) ([]vault.Material, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	if cfg.OpenRouterAPIKey == "" {
		return nil, fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup'")
	}
	if opts.ImageModel == "" {
		opts.ImageModel = orDefault(cfg.ModelImage, ai.DefaultModel)
	}
	if opts.TriageModel == "" {
		opts.TriageModel = orDefault(cfg.ModelTriage, ai.DefaultTriageModel)
	}
	parents, err := s.materials(ctx, ids)
	if err != nil {
		return nil, err
	}
	run, err := s.Vault.StartRun(ctx, s.CollectionID, "colorize", map[string]string{"image_model": opts.ImageModel, "triage_model": opts.TriageModel}, materialIDs(parents))
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "caramel-vault-colorize-")
	if err != nil {
		_ = s.Vault.FinishRun(ctx, run.ID, "failed", nil, err)
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	attempts := ai.NewAttemptCollector()
	persistCtx := context.Background()
	var materials []vault.Material
	paths := make([]string, 0, len(parents))
	for _, parent := range parents {
		paths = append(paths, s.Vault.ObjectPath(parent))
	}
	batchResults, batchErr := ai.ColorizeImagesContext(ctx, paths, ai.ColorizeOptions{
		OutputDir: tempDir, APIKey: cfg.OpenRouterAPIKey, Model: opts.ImageModel,
		ModelFallbacks: cfg.ModelImageFallbacks, TriageModel: opts.TriageModel,
		TriageModelFallbacks: cfg.ModelTriageFallbacks, DisableTriage: opts.DisableTriage, MaxWorkers: opts.MaxWorkers,
		AttemptWriter: attempts.Writer(),
	}, func(event ai.BatchProgressEvent) {
		if event.State == "started" && event.Index >= 0 && event.Index < len(parents) {
			parent := parents[event.Index]
			s.emit(ProgressEvent{Step: "colorizing", Current: event.Index, Total: event.Total, Message: parent.Title, Path: paths[event.Index]})
		}
	})
	var failures []string
	for i, batch := range batchResults {
		parent := parents[i]
		if batch.Err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", parent.Title, batch.Err))
			continue
		}
		if batch.Result == nil {
			failures = append(failures, fmt.Sprintf("%s: resultado vazio", parent.Title))
			continue
		}
		if batch.Result.Skipped {
			s.emit(ProgressEvent{Step: "skipped", Current: i + 1, Total: len(parents), Message: fmt.Sprintf("%s: %s", parent.Title, batch.Result.SkipReason)})
			continue
		}

		imported, importErr := s.Vault.ImportFile(persistCtx, batch.Result.ColorizedPath, parent.Title+" colorida", []string{"colorido"})
		if importErr != nil {
			failures = append(failures, fmt.Sprintf("%s: falha ao importar resultado: %v", parent.Title, importErr))
			continue
		}
		materials = append(materials, imported.Material)
		attempts.AnnotateItem(i+1, imported.Material.ObjectPath, false)
		if err := s.Vault.AddDerivation(persistCtx, run.ID, []string{parent.ID}, []string{imported.Material.ID}); err != nil {
			failures = append(failures, fmt.Sprintf("%s: falha ao registrar derivação: %v", parent.Title, err))
			continue
		}
		if s.CollectionID != "" {
			if err := s.Vault.AddToCollection(persistCtx, s.CollectionID, imported.Material.ID); err != nil {
				failures = append(failures, fmt.Sprintf("%s: falha ao adicionar à coleção: %v", parent.Title, err))
				continue
			}
		}
		s.emit(ProgressEvent{Step: "saved", Current: i + 1, Total: len(parents), Message: imported.Material.Title})
	}
	if batchErr != nil {
		status := "failed"
		if ctx.Err() != nil {
			status = "canceled"
		}
		_ = s.Vault.FinishRunWithAttempts(persistCtx, run.ID, status, materialIDs(materials), batchErr, attempts.Snapshot())
		return materials, batchErr
	}
	if len(failures) > 0 {
		batchErr = fmt.Errorf("%d material(is) falharam: %s", len(failures), strings.Join(failures, "; "))
		_ = s.Vault.FinishRunWithAttempts(persistCtx, run.ID, "failed", materialIDs(materials), batchErr, attempts.Snapshot())
		return materials, batchErr
	}
	if err := s.Vault.FinishRunWithAttempts(persistCtx, run.ID, "completed", materialIDs(materials), nil, attempts.Snapshot()); err != nil {
		return materials, err
	}
	return materials, nil
}

// Cards gera uma ficha e importa o PDF como material derivado.
func (s *VaultImageService) Cards(ctx context.Context, ids []string) ([]vault.Material, error) {
	return s.print(ctx, "cards", ids, func(paths []string, output string) error {
		items := make([]cards.CardItem, 0, len(paths))
		for _, path := range paths {
			items = append(items, cards.CardItem{Name: displayName(filepath.Base(path)), ImagePath: path})
		}
		opts := cards.DefaultOptions()
		return cards.GenerateCardsPDF(items, output, opts)
	})
}

// TwoUp gera um PDF 2-up e importa o resultado como material derivado.
func (s *VaultImageService) TwoUp(ctx context.Context, ids []string) ([]vault.Material, error) {
	return s.print(ctx, "2up", ids, func(paths []string, output string) error {
		return pdf.Generate2UpPDF(paths, output, pdf.DefaultOptions())
	})
}

func (s *VaultImageService) print(ctx context.Context, operation string, ids []string, generate func([]string, string) error) ([]vault.Material, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	parents, err := s.materials(ctx, ids)
	if err != nil {
		return nil, err
	}
	run, err := s.Vault.StartRun(ctx, s.CollectionID, operation, nil, materialIDs(parents))
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "caramel-vault-print-")
	if err != nil {
		_ = s.Vault.FinishRun(ctx, run.ID, "failed", nil, err)
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	paths := make([]string, 0, len(parents))
	for _, parent := range parents {
		paths = append(paths, s.Vault.ObjectPath(parent))
	}
	output := filepath.Join(tempDir, fmt.Sprintf("%s-%d.pdf", operation, time.Now().UnixNano()))
	s.emit(ProgressEvent{Step: operation, Current: 0, Total: 1, Message: "Gerando material de impressão..."})
	if err := generate(paths, output); err != nil {
		_ = s.Vault.FinishRun(ctx, run.ID, "failed", nil, err)
		return nil, err
	}
	imported, err := s.Vault.ImportFile(ctx, output, operation, []string{"impressao", operation})
	if err != nil {
		_ = s.Vault.FinishRun(ctx, run.ID, "failed", nil, err)
		return nil, err
	}
	_ = s.Vault.AddDerivation(ctx, run.ID, materialIDs(parents), []string{imported.Material.ID})
	if s.CollectionID != "" {
		_ = s.Vault.AddToCollection(ctx, s.CollectionID, imported.Material.ID)
	}
	if err := s.Vault.FinishRun(ctx, run.ID, "completed", []string{imported.Material.ID}, nil); err != nil {
		return nil, err
	}
	s.emit(ProgressEvent{Step: "done", Current: 1, Total: 1, Message: imported.Material.Title})
	return []vault.Material{imported.Material}, nil
}

func (s *VaultImageService) materials(ctx context.Context, ids []string) ([]vault.Material, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("selecione pelo menos um material")
	}
	result := make([]vault.Material, 0, len(ids))
	for _, id := range ids {
		material, err := s.Vault.GetMaterial(ctx, id)
		if err != nil {
			return nil, err
		}
		if material.Kind != "image" {
			return nil, fmt.Errorf("material '%s' não é uma imagem", material.Title)
		}
		result = append(result, *material)
	}
	return result, nil
}

func (s *VaultImageService) validate() error {
	if s == nil || s.Vault == nil {
		return fmt.Errorf("vault não está aberto")
	}
	return nil
}

func (s *VaultImageService) emit(event ProgressEvent) {
	if s.Progress != nil {
		s.Progress(normalizeProgressEvent(event))
	}
}

func materialIDs(materials []vault.Material) []string {
	ids := make([]string, 0, len(materials))
	for _, material := range materials {
		ids = append(ids, material.ID)
	}
	return ids
}
