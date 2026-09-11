package pipeline

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
)

// PipelineResult guarda o resumo da execução do pipeline automatizado
type PipelineResult struct {
	DocxPath           string
	OutputDir          string
	RebuiltDocxPath    string
	TotalExtracted     int
	TotalSkipped       int
	TotalColorized     int
	TotalTriageSkipped int
	TotalFormatSkipped int
	Results            []ai.ColorizeResult
	SkippedImages      []docx.ExtractedImage
	TriageSkipped      []ai.TriageSkipInfo
	FormatSkipped      []docx.ExtractedImage
	Warnings           []string
}

// PipelineOptions controla os diagnósticos do pipeline sem acoplar a ferramenta à CLI.
type PipelineOptions struct {
	Verbose          bool
	DiagnosticWriter io.Writer
}

// RunDocxPipeline executa o fluxo completo:
//  1. Extração automática de todas as imagens contidas no .docx (filtrando por tamanho mínimo)
//  2. Sanitização automática do nome do diretório de destino
//  3. Triagem de economia por imagem (análise local de saturação + LLM de visão barata),
//     pulando imagens que não precisam de coloração (fail-open em caso de erro)
//  4. Coloração de cada imagem aprovada via IA (OpenRouter / Nano Banana 2)
//  5. Redimensionamento para o tamanho original das imagens
//  6. Reconstrução de um novo arquivo .docx com as imagens substituídas
func RunDocxPipeline(docxPath string, outputDir string, apiKey string, model string, minSizeBytes int64, verbose bool, triageModel string, noTriage bool) (*PipelineResult, error) {
	return RunDocxPipelineWithOptions(docxPath, outputDir, apiKey, model, minSizeBytes, PipelineOptions{Verbose: verbose}, triageModel, noTriage)
}

// RunDocxPipelineWithOptions executa o pipeline com diagnóstico direcionável.
func RunDocxPipelineWithOptions(docxPath string, outputDir string, apiKey string, model string, minSizeBytes int64, options PipelineOptions, triageModel string, noTriage bool) (*PipelineResult, error) {
	targetDir := outputDir
	if targetDir == "" {
		targetDir = docx.SanitizeFolderName(docxPath)
	}

	// 1. Extrai as imagens originais para uma pasta temporária de trabalho (aplicando filtro de tamanho mínimo)
	tempExtractDir := filepath.Join(targetDir, ".temp_raw_images")
	extractRes, err := docx.ExtractImagesFiltered(docxPath, tempExtractDir, minSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("erro na extração inicial de imagens do .docx: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	if extractRes.TotalExtracted == 0 {
		return &PipelineResult{
			DocxPath:      docxPath,
			OutputDir:     targetDir,
			TotalSkipped:  extractRes.TotalSkipped,
			SkippedImages: extractRes.SkippedImages,
		}, nil
	}

	// 2. Colora cada imagem mantida e redimensiona para a dimensão original
	colorizeOpts := ai.ColorizeOptions{
		OutputDir:        targetDir,
		APIKey:           apiKey,
		Model:            model,
		TriageModel:      triageModel,
		DisableTriage:    noTriage,
		Verbose:          options.Verbose,
		DiagnosticWriter: options.DiagnosticWriter,
	}

	var colorizedResults []ai.ColorizeResult
	var triageSkipped []ai.TriageSkipInfo
	var formatSkipped []docx.ExtractedImage
	var warnings []string
	replacements := make(map[string][]byte)

	for _, img := range extractRes.Images {
		// Pula formatos não coloríveis (emf, wmf, bin, svg...) sem gastar a API
		if !docx.IsColorableFormat(img.Format) {
			warning := fmt.Sprintf("%s: formato não colorível (%s)", img.OriginalName, img.Format)
			diagnosticf(options, "⏭️  Pulada: %s\n", warning)
			formatSkipped = append(formatSkipped, img)
			continue
		}

		imgPath := filepath.Join(tempExtractDir, img.OriginalName)
		res, err := ai.ColorizeSingleImage(imgPath, colorizeOpts)
		if err != nil {
			warning := fmt.Sprintf("não foi possível colorir '%s': %v", img.OriginalName, err)
			warnings = append(warnings, warning)
			diagnosticf(options, "⚠️ Aviso: %s\n", warning)
			continue
		}

		// Imagem rejeitada pela triagem de economia: não colorida nem substituída no docx
		if res.Skipped {
			diagnosticf(options, "⏭️  Pulada pela triagem: %s (%s)\n", img.OriginalName, res.SkipReason)
			triageSkipped = append(triageSkipped, ai.TriageSkipInfo{
				Name:   img.OriginalName,
				Stage:  res.SkipStage,
				Reason: res.SkipReason,
			})
			continue
		}

		// Redimensiona a imagem gerada para ter exatamente os mesmos pixels da original
		resizedBytes, err := docx.ResizeToMatch(imgPath, res.ColorizedPath)
		if err != nil {
			warning := fmt.Sprintf("falha ao ajustar tamanho da imagem colorida '%s': %v", img.OriginalName, err)
			warnings = append(warnings, warning)
			diagnosticf(options, "⚠️ Aviso: %s\n", warning)
			continue
		}

		// Adiciona a imagem redimensionada ao mapa de substituição do zip
		replacements[img.PathInZip] = resizedBytes
		colorizedResults = append(colorizedResults, *res)
	}

	// 3. Reconstrói um novo arquivo .docx com as imagens substituídas
	baseName := strings.TrimSuffix(filepath.Base(docxPath), filepath.Ext(docxPath))
	rebuiltDocxName := fmt.Sprintf("%s colorida.docx", baseName)
	rebuiltDocxPath := filepath.Join(targetDir, rebuiltDocxName)

	if len(replacements) > 0 {
		if err := docx.RebuildDocx(docxPath, rebuiltDocxPath, replacements); err != nil {
			return nil, fmt.Errorf("erro ao reconstruir arquivo docx colorida: %w", err)
		}
	}

	return &PipelineResult{
		DocxPath:           docxPath,
		OutputDir:          targetDir,
		RebuiltDocxPath:    rebuiltDocxPath,
		TotalExtracted:     extractRes.TotalExtracted,
		TotalSkipped:       extractRes.TotalSkipped,
		TotalColorized:     len(colorizedResults),
		TotalTriageSkipped: len(triageSkipped),
		TotalFormatSkipped: len(formatSkipped),
		Results:            colorizedResults,
		TriageSkipped:      triageSkipped,
		FormatSkipped:      formatSkipped,
		Warnings:           warnings,
	}, nil
}

// RunDocxPipelineSelected executa o pipeline apenas nas imagens pré-selecionadas pelo usuário
func RunDocxPipelineSelected(docxPath string, outputDir string, apiKey string, model string, selectedImages []docx.ExtractedImage, verbose bool, triageModel string, noTriage bool) (*PipelineResult, error) {
	return RunDocxPipelineSelectedWithOptions(docxPath, outputDir, apiKey, model, selectedImages, PipelineOptions{Verbose: verbose}, triageModel, noTriage)
}

// RunDocxPipelineSelectedWithOptions executa o pipeline interativo com diagnóstico direcionável.
func RunDocxPipelineSelectedWithOptions(docxPath string, outputDir string, apiKey string, model string, selectedImages []docx.ExtractedImage, options PipelineOptions, triageModel string, noTriage bool) (*PipelineResult, error) {
	targetDir := outputDir
	if targetDir == "" {
		targetDir = docx.SanitizeFolderName(docxPath)
	}

	if len(selectedImages) == 0 {
		return &PipelineResult{
			DocxPath:  docxPath,
			OutputDir: targetDir,
		}, nil
	}

	// 1. Extrai apenas as imagens selecionadas para uma pasta temporária de trabalho
	tempExtractDir := filepath.Join(targetDir, ".temp_raw_images")
	extractRes, err := docx.ExtractImagesFromList(docxPath, tempExtractDir, selectedImages)
	if err != nil {
		return nil, fmt.Errorf("erro na extração de imagens selecionadas: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	// 2. Colora cada imagem selecionada e redimensiona para a dimensão original
	colorizeOpts := ai.ColorizeOptions{
		OutputDir:        targetDir,
		APIKey:           apiKey,
		Model:            model,
		TriageModel:      triageModel,
		DisableTriage:    noTriage,
		Verbose:          options.Verbose,
		DiagnosticWriter: options.DiagnosticWriter,
	}

	var colorizedResults []ai.ColorizeResult
	var triageSkipped []ai.TriageSkipInfo
	var formatSkipped []docx.ExtractedImage
	var warnings []string
	replacements := make(map[string][]byte)

	for _, img := range extractRes.Images {
		// Pula formatos não coloríveis (emf, wmf, bin, svg...) sem gastar a API
		if !docx.IsColorableFormat(img.Format) {
			warning := fmt.Sprintf("%s: formato não colorível (%s)", img.OriginalName, img.Format)
			diagnosticf(options, "⏭️  Pulada: %s\n", warning)
			formatSkipped = append(formatSkipped, img)
			continue
		}

		imgPath := filepath.Join(tempExtractDir, img.OriginalName)
		res, err := ai.ColorizeSingleImage(imgPath, colorizeOpts)
		if err != nil {
			warning := fmt.Sprintf("não foi possível colorir '%s': %v", img.OriginalName, err)
			warnings = append(warnings, warning)
			diagnosticf(options, "⚠️ Aviso: %s\n", warning)
			continue
		}

		// Imagem rejeitada pela triagem de economia: não colorida nem substituída no docx
		if res.Skipped {
			diagnosticf(options, "⏭️  Pulada pela triagem: %s (%s)\n", img.OriginalName, res.SkipReason)
			triageSkipped = append(triageSkipped, ai.TriageSkipInfo{
				Name:   img.OriginalName,
				Stage:  res.SkipStage,
				Reason: res.SkipReason,
			})
			continue
		}

		// Redimensiona a imagem gerada para ter exatamente os mesmos pixels da original
		resizedBytes, err := docx.ResizeToMatch(imgPath, res.ColorizedPath)
		if err != nil {
			warning := fmt.Sprintf("falha ao ajustar tamanho da imagem colorida '%s': %v", img.OriginalName, err)
			warnings = append(warnings, warning)
			diagnosticf(options, "⚠️ Aviso: %s\n", warning)
			continue
		}

		replacements[img.PathInZip] = resizedBytes
		colorizedResults = append(colorizedResults, *res)
	}

	// 3. Reconstrói o arquivo .docx com as imagens substituídas
	baseName := strings.TrimSuffix(filepath.Base(docxPath), filepath.Ext(docxPath))
	rebuiltDocxName := fmt.Sprintf("%s colorida.docx", baseName)
	rebuiltDocxPath := filepath.Join(targetDir, rebuiltDocxName)

	if len(replacements) > 0 {
		if err := docx.RebuildDocx(docxPath, rebuiltDocxPath, replacements); err != nil {
			return nil, fmt.Errorf("erro ao reconstruir arquivo docx colorida: %w", err)
		}
	}

	return &PipelineResult{
		DocxPath:           docxPath,
		OutputDir:          targetDir,
		RebuiltDocxPath:    rebuiltDocxPath,
		TotalExtracted:     extractRes.TotalExtracted,
		TotalColorized:     len(colorizedResults),
		TotalTriageSkipped: len(triageSkipped),
		TotalFormatSkipped: len(formatSkipped),
		Results:            colorizedResults,
		TriageSkipped:      triageSkipped,
		FormatSkipped:      formatSkipped,
		Warnings:           warnings,
	}, nil
}

func diagnosticf(options PipelineOptions, format string, args ...interface{}) {
	if !options.Verbose || options.DiagnosticWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(options.DiagnosticWriter, format, args...)
}
