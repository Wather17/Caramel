package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/output"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
	"caramel/internal/tools/pdf"
	"caramel/internal/tools/pipeline"
	"caramel/internal/ui"
)

// ProcessDocxOptions contém os parâmetros para execução do pipeline de processamento e reconstrução de .docx
type ProcessDocxOptions struct {
	DocxPath    string
	OutputDir   string
	ModelName   string
	MinSize     string
	Interactive bool
	Verbose     bool
	TriageModel string // Modelo de visão usado na triagem (vazio = padrão gratuito)
	NoTriage    bool   // true desativa a triagem e colora todas as imagens elegíveis
	Output      output.Options
	Out         io.Writer
	Err         io.Writer
}

// RunProcessDocx executa o fluxo completo do pipeline DOCX (interativo ou automatizado)
func RunProcessDocx(opts ProcessDocxOptions) error {
	if opts.Verbose {
		opts.Output.Verbose = true
	}
	stdout, stderr := opts.Out, opts.Err
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	renderer, err := output.New(opts.Output, stdout, stderr)
	if err != nil {
		return err
	}

	docxPath := opts.DocxPath

	if !strings.HasSuffix(strings.ToLower(docxPath), ".docx") {
		return fmt.Errorf("o arquivo '%s' não possui a extensão .docx", docxPath)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	if cfg.OpenRouterAPIKey == "" {
		return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
	}

	modelName := opts.ModelName
	if modelName == "" {
		modelName = ai.DefaultModel
	}

	minSizeStr := opts.MinSize
	if minSizeStr == "" {
		minSizeStr = "0"
	}

	minSizeBytes, err := docx.ParseSizeInBytes(minSizeStr)
	if err != nil {
		return err
	}

	// Modo Interativo (--interactive / -i) com preview ANSI TrueColor no terminal
	if opts.Interactive {
		if opts.Output.JSON {
			return fmt.Errorf("o modo --json não pode ser combinado com --interactive")
		}
		allImages, err := docx.ListImages(docxPath)
		if err != nil {
			return err
		}

		if len(allImages) == 0 {
			return renderer.Result(output.Result{Status: output.StateWarning, Summary: fmt.Sprintf("Nenhuma imagem foi encontrada em %s.", filepath.Base(docxPath))})
		}

		keptImages, _ := docx.FilterImagesByMinSize(allImages, minSizeBytes)
		if len(keptImages) == 0 {
			return renderer.Result(output.Result{Status: output.StateWarning, Summary: fmt.Sprintf("Nenhuma imagem atende ao tamanho mínimo de %s.", minSizeStr)})
		}

		// Extrai imagens elegíveis para pasta temporária para gerar os previews ANSI
		tempExtractDir := filepath.Join(os.TempDir(), "caramel_process_preview_"+strings.TrimSuffix(filepath.Base(docxPath), filepath.Ext(docxPath)))
		defer os.RemoveAll(tempExtractDir)

		_, err = docx.ExtractImagesFromList(docxPath, tempExtractDir, keptImages)
		if err != nil {
			return fmt.Errorf("falha ao preparar imagens para preview: %w", err)
		}

		var candidatePaths []string
		imgMap := make(map[string]docx.ExtractedImage)
		for _, img := range keptImages {
			fullPath := filepath.Join(tempExtractDir, img.OriginalName)
			candidatePaths = append(candidatePaths, fullPath)
			imgMap[fullPath] = img
		}

		// Ordena numericamente (natural sort)
		pdf.SortNatural(candidatePaths)

		selectedPaths, err := ui.SelectImageFilesWithPreviewInteractive(candidatePaths)
		if err != nil {
			return err
		}

		if len(selectedPaths) == 0 {
			return renderer.Result(output.Result{Status: output.StateCanceled, Summary: "Nenhuma imagem foi selecionada."})
		}

		selectedImages := make([]docx.ExtractedImage, 0, len(selectedPaths))
		for _, p := range selectedPaths {
			if imgInfo, ok := imgMap[p]; ok {
				selectedImages = append(selectedImages, imgInfo)
			}
		}

		res, err := pipeline.RunDocxPipelineSelectedWithOptions(docxPath, opts.OutputDir, cfg.OpenRouterAPIKey, modelName, selectedImages, pipeline.PipelineOptions{Verbose: opts.Output.Verbose, DiagnosticWriter: stderr}, opts.TriageModel, opts.NoTriage)
		if err != nil {
			return err
		}
		return renderDocxPipelineResult(renderer, res, true)
	}

	// Execução Automatizada Padrão (Colora todas as imagens mantidas pelo filtro minSize)
	res, err := pipeline.RunDocxPipelineWithOptions(docxPath, opts.OutputDir, cfg.OpenRouterAPIKey, modelName, minSizeBytes, pipeline.PipelineOptions{Verbose: opts.Output.Verbose, DiagnosticWriter: stderr}, opts.TriageModel, opts.NoTriage)
	if err != nil {
		return err
	}
	return renderDocxPipelineResult(renderer, res, false)
}

func renderDocxPipelineResult(renderer *output.Renderer, res *pipeline.PipelineResult, selected bool) error {
	if res == nil {
		return fmt.Errorf("pipeline DOCX não retornou resultado")
	}
	warnings := make([]string, 0, 3)
	if len(res.Warnings) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d item(ns) não puderam ser processado(s)", len(res.Warnings)))
	}
	if res.TotalSkipped > 0 {
		warnings = append(warnings, fmt.Sprintf("%d imagem(ns) ignorada(s) pelo filtro de tamanho", res.TotalSkipped))
	}
	if res.TotalFormatSkipped > 0 {
		warnings = append(warnings, fmt.Sprintf("%d imagem(ns) ignorada(s) por formato não colorível", res.TotalFormatSkipped))
	}
	if res.TotalTriageSkipped > 0 {
		warnings = append(warnings, fmt.Sprintf("%d imagem(ns) ignorada(s) pela triagem", res.TotalTriageSkipped))
	}
	if renderer.Options().Verbose {
		for _, warning := range res.Warnings {
			renderer.Diagnostic("⚠️ %s\n", warning)
		}
		for _, skipped := range res.SkippedImages {
			renderer.Diagnostic("⏭️ filtro: %s\n", skipped.OriginalName)
		}
		for _, skipped := range res.TriageSkipped {
			renderer.Diagnostic("⏭️ triagem: %s [%s] %s\n", skipped.Name, skipped.Stage, skipped.Reason)
		}
	}

	status := output.StateSuccess
	summary := fmt.Sprintf("Pipeline concluído: %d imagem(ns) colorida(s).", res.TotalColorized)
	if res.TotalColorized == 0 {
		status = output.StateWarning
		switch {
		case res.TotalExtracted == 0 && res.TotalSkipped > 0:
			summary = "Nenhuma imagem atende ao filtro de tamanho."
		case res.TotalExtracted == 0:
			summary = "Nenhuma imagem foi encontrada no documento."
		case res.TotalTriageSkipped == res.TotalExtracted:
			summary = "Nenhuma imagem foi aprovada pela triagem."
		default:
			summary = "Nenhuma imagem foi colorida; consulte os avisos para entender o motivo."
		}
	}
	if selected && res.TotalColorized > 0 {
		summary = fmt.Sprintf("Processamento concluído: %d imagem(ns) colorida(s).", res.TotalColorized)
	}
	outputs := []string{}
	rebuiltDocxPath := res.RebuiltDocxPath
	if rebuiltDocxPath != "" {
		if _, err := os.Stat(rebuiltDocxPath); err != nil {
			rebuiltDocxPath = ""
		}
	}
	if rebuiltDocxPath != "" {
		outputs = append(outputs, rebuiltDocxPath)
	}
	if res.OutputDir != "" {
		outputs = append(outputs, res.OutputDir)
	}
	return renderer.Result(output.Result{Status: status, Summary: summary, Count: res.TotalColorized, Outputs: outputs, Warnings: warnings})
}
