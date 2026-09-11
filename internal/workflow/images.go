// Package workflow coordena os fluxos de imagem usados pela CLI e pela TUI.
package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"caramel/internal/config"
	"caramel/internal/output"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/cards"
	"caramel/internal/tools/pdf"
	"caramel/internal/workspace"
)

// ProgressEvent preserva o nome histórico, usando o contrato compartilhado de output.
type ProgressEvent = output.Event

// ImageOptions concentra opções da primeira versão dos fluxos de imagem.
type ImageOptions struct {
	Items         []string
	Theme         string
	Count         int
	Style         string
	Aspect        string
	ImageModel    string
	TextModel     string
	TriageModel   string
	DisableTriage bool
}

// ImageService executa operações no contexto de um projeto persistente.
type ImageService struct {
	Project  *workspace.Project
	Progress func(ProgressEvent)
}

// Generate cria imagens no diretório outputs/generated/<run>.
func (s *ImageService) Generate(ctx context.Context, opts ImageOptions) ([]workspace.Artifact, error) {
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

	run, err := s.Project.BeginRun("generate", nil, map[string]string{
		"style": opts.Style, "aspect": opts.Aspect, "image_model": opts.ImageModel, "text_model": opts.TextModel,
	})
	if err != nil {
		return nil, err
	}
	outputDir := s.Project.OutputDir("generated", run.ID)
	client, err := ai.NewClient(cfg.OpenRouterAPIKey)
	if err != nil {
		_ = s.Project.FinishRun(run.ID, "failed", nil, err)
		return nil, err
	}
	harnessCfg := ai.HarnessConfig{
		Items: opts.Items, Theme: opts.Theme, Count: opts.Count, Style: opts.Style,
		OutputDir: outputDir, TextModel: opts.TextModel, ImageModel: opts.ImageModel, Aspect: opts.Aspect,
	}
	s.emit(ProgressEvent{Step: "synthesizing", Message: "Sintetizando prompts..."})
	items, err := ai.SynthesizePrompts(harnessCfg, client)
	if err != nil {
		_ = s.Project.FinishRun(run.ID, "failed", nil, err)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = s.Project.FinishRun(run.ID, "canceled", nil, err)
		return nil, err
	}

	results, err := ai.ExecuteGenerationHarnessContext(ctx, items, harnessCfg, client, func(ev ai.HarnessProgressEvent) {
		message := ev.Item.Name
		if ev.Item.Error != "" {
			message = ev.Item.Error
		}
		s.emit(ProgressEvent{Step: ev.CurrentStep, Current: ev.Completed, Total: ev.Total, Message: message, Path: ev.Item.ImagePath})
	})
	if err != nil {
		status := "failed"
		if ctx.Err() != nil {
			status = "canceled"
		}
		_ = s.Project.FinishRun(run.ID, status, nil, err)
		return nil, err
	}

	artifacts := make([]workspace.Artifact, 0)
	for _, result := range results {
		if result.Status != "done" || result.ImagePath == "" {
			continue
		}
		artifact, err := s.artifactFromPath(result.ImagePath, "generate")
		if err != nil {
			continue
		}
		artifacts = append(artifacts, artifact)
	}
	if err := s.Project.FinishRun(run.ID, "completed", artifacts, nil); err != nil {
		return artifacts, err
	}
	s.emit(ProgressEvent{Step: "done", Current: len(artifacts), Total: len(results), Message: fmt.Sprintf("%d imagem(ns) gerada(s)", len(artifacts))})
	return artifacts, nil
}

// Colorize colore os ativos escolhidos, preservando resultados parciais.
func (s *ImageService) Colorize(ctx context.Context, assetIDs []string, opts ImageOptions) ([]workspace.Artifact, error) {
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
	assets := s.Project.ImageFiles()
	selected := selectImageFiles(assets, assetIDs)
	if len(selected) == 0 {
		return nil, fmt.Errorf("nenhuma imagem selecionada para colorização")
	}
	ids := idsOf(selected)
	run, err := s.Project.BeginRun("colorize", ids, map[string]string{"image_model": opts.ImageModel, "triage_model": opts.TriageModel})
	if err != nil {
		return nil, err
	}
	outputDir := s.Project.OutputDir("colorized", run.ID)
	artifacts := make([]workspace.Artifact, 0)
	var failures []string
	for i, asset := range selected {
		if err := ctx.Err(); err != nil {
			_ = s.Project.FinishRun(run.ID, "canceled", artifacts, err)
			return artifacts, err
		}
		path := s.Project.Resolve(asset.Path)
		s.emit(ProgressEvent{Step: "colorizing", Current: i, Total: len(selected), Message: asset.Name, Path: path})
		result, colorErr := ai.ColorizeSingleImage(path, ai.ColorizeOptions{
			OutputDir: outputDir, APIKey: cfg.OpenRouterAPIKey, Model: opts.ImageModel,
			TriageModel: opts.TriageModel, DisableTriage: opts.DisableTriage,
		})
		if colorErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", asset.Name, colorErr))
			continue
		}
		if result.Skipped {
			s.emit(ProgressEvent{Step: "skipped", Current: i + 1, Total: len(selected), Message: fmt.Sprintf("%s: %s", asset.Name, result.SkipReason)})
			continue
		}
		artifact, artifactErr := s.artifactFromPath(result.ColorizedPath, "colorize")
		if artifactErr == nil {
			artifacts = append(artifacts, artifact)
		}
		s.emit(ProgressEvent{Step: "saved", Current: i + 1, Total: len(selected), Message: asset.Name, Path: result.ColorizedPath})
	}
	if len(failures) > 0 {
		err = fmt.Errorf("%d imagem(ns) falharam: %s", len(failures), strings.Join(failures, "; "))
		_ = s.Project.FinishRun(run.ID, "failed", artifacts, err)
		return artifacts, err
	}
	if err := s.Project.FinishRun(run.ID, "completed", artifacts, nil); err != nil {
		return artifacts, err
	}
	s.emit(ProgressEvent{Step: "done", Current: len(selected), Total: len(selected), Message: fmt.Sprintf("%d imagem(ns) processada(s)", len(artifacts))})
	return artifacts, nil
}

// Cards gera fichas A4 para os ativos selecionados.
func (s *ImageService) Cards(ctx context.Context, assetIDs []string) ([]workspace.Artifact, error) {
	return s.print(ctx, "cards", assetIDs, func(paths []string, output string) error {
		items := make([]cards.CardItem, 0, len(paths))
		for _, path := range paths {
			items = append(items, cards.CardItem{Name: displayName(filepath.Base(path)), ImagePath: path})
		}
		opts := cards.DefaultOptions()
		opts.Title = s.Project.Name
		return cards.GenerateCardsPDF(items, output, opts)
	})
}

// TwoUp gera um PDF A4 paisagem com duas imagens por folha.
func (s *ImageService) TwoUp(ctx context.Context, assetIDs []string) ([]workspace.Artifact, error) {
	return s.print(ctx, "2up", assetIDs, func(paths []string, output string) error {
		opts := pdf.DefaultOptions()
		return pdf.Generate2UpPDF(paths, output, opts)
	})
}

func (s *ImageService) print(ctx context.Context, step string, assetIDs []string, generate func([]string, string) error) ([]workspace.Artifact, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	selected := selectImageFiles(s.Project.ImageFiles(), assetIDs)
	if len(selected) == 0 {
		return nil, fmt.Errorf("nenhuma imagem selecionada para gerar %s", step)
	}
	paths := make([]string, 0, len(selected))
	ids := make([]string, 0, len(selected))
	for _, asset := range selected {
		paths = append(paths, s.Project.Resolve(asset.Path))
		ids = append(ids, asset.ID)
	}
	run, err := s.Project.BeginRun(step, ids, nil)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = s.Project.FinishRun(run.ID, "canceled", nil, err)
		return nil, err
	}
	s.emit(ProgressEvent{Step: step, Current: 0, Total: 1, Message: fmt.Sprintf("Gerando %s...", step)})
	outputDir := s.Project.OutputDir(step, run.ID)
	ext := "pdf"
	name := fmt.Sprintf("%s_%s_%s.%s", sanitizeName(s.Project.Name), step, time.Now().UTC().Format("20060102-150405"), ext)
	output := filepath.Join(outputDir, name)
	if err := generate(paths, output); err != nil {
		_ = s.Project.FinishRun(run.ID, "failed", nil, err)
		return nil, err
	}
	artifact, err := s.artifactFromPath(output, step)
	if err != nil {
		_ = s.Project.FinishRun(run.ID, "failed", nil, err)
		return nil, err
	}
	if err := s.Project.FinishRun(run.ID, "completed", []workspace.Artifact{artifact}, nil); err != nil {
		return nil, err
	}
	s.emit(ProgressEvent{Step: "done", Current: 1, Total: 1, Message: fmt.Sprintf("Arquivo gerado: %s", artifact.Name), Path: output})
	return []workspace.Artifact{artifact}, nil
}

func (s *ImageService) artifactFromPath(path, step string) (workspace.Artifact, error) {
	relative, err := filepath.Rel(s.Project.Directory, path)
	if err != nil {
		return workspace.Artifact{}, err
	}
	kind := "file"
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
		kind = "image"
	} else if ext == ".pdf" {
		kind = "pdf"
	} else if ext == ".html" {
		kind = "html"
	}
	return workspace.Artifact{ID: fmt.Sprintf("%s-%d", step, time.Now().UnixNano()), Name: filepath.Base(path), Path: filepath.ToSlash(relative), Kind: kind, Step: step, CreatedAt: time.Now().UTC()}, nil
}

func (s *ImageService) validate() error {
	if s == nil || s.Project == nil {
		return fmt.Errorf("projeto não está carregado")
	}
	return nil
}

func (s *ImageService) emit(event ProgressEvent) {
	if s.Progress != nil {
		s.Progress(normalizeProgressEvent(event))
	}
}

func normalizeProgressEvent(event ProgressEvent) ProgressEvent {
	if event.State != "" {
		return event
	}
	switch event.Step {
	case "skipped":
		event.State = output.StateSkipped
	case "done", "saved":
		event.State = output.StateSuccess
	case "error":
		event.State = output.StateFailed
	case "canceled", "cancelled":
		event.State = output.StateCanceled
	}
	return event
}

func selectImageFiles(all []workspace.ImageFile, ids []string) []workspace.ImageFile {
	if len(ids) == 0 {
		return all
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	var selected []workspace.ImageFile
	for _, asset := range all {
		if wanted[asset.ID] {
			selected = append(selected, asset)
		}
	}
	return selected
}

func idsOf(files []workspace.ImageFile) []string {
	ids := make([]string, 0, len(files))
	for _, file := range files {
		ids = append(ids, file.ID)
	}
	return ids
}

func displayName(file string) string {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	base = strings.TrimLeft(base, "0123456789")
	base = strings.Trim(base, "_ -")
	base = strings.NewReplacer("_", " ", "-", " ").Replace(base)
	if base == "" {
		return file
	}
	return strings.Title(base)
}

func sanitizeName(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
