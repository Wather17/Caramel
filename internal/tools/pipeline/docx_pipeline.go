package pipeline

import (
	"context"
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
	Context          context.Context
	MaxWorkers       int
	Verbose          bool
	DiagnosticWriter io.Writer
}

type imageBatchProcessingResult struct {
	colorizedResults []ai.ColorizeResult
	triageSkipped    []ai.TriageSkipInfo
	formatSkipped    []docx.ExtractedImage
	warnings         []string
	replacements     map[string][]byte
}

type imagePipelineItemResult struct {
	result       *ai.ColorizeResult
	resizedBytes []byte
	err          error
}

func processExtractedImages(ctx context.Context, tempExtractDir, targetDir, apiKey, model string, images []docx.ExtractedImage, options PipelineOptions, triageModel string, noTriage bool) (imageBatchProcessingResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	processed := imageBatchProcessingResult{replacements: make(map[string][]byte)}
	eligible := make([]docx.ExtractedImage, 0, len(images))
	paths := make([]string, 0, len(images))
	for _, img := range images {
		if !docx.IsColorableFormat(img.Format) {
			warning := fmt.Sprintf("%s: formato não colorível (%s)", img.OriginalName, img.Format)
			diagnosticf(options, "⏭️  Pulada: %s\n", warning)
			processed.formatSkipped = append(processed.formatSkipped, img)
			continue
		}
		eligible = append(eligible, img)
		paths = append(paths, filepath.Join(tempExtractDir, img.OriginalName))
	}

	colorizeOpts := ai.ColorizeOptions{
		OutputDir:        targetDir,
		APIKey:           apiKey,
		Model:            model,
		TriageModel:      triageModel,
		DisableTriage:    noTriage,
		Verbose:          options.Verbose,
		DiagnosticWriter: options.DiagnosticWriter,
	}
	itemResults := make([]imagePipelineItemResult, len(eligible))
	itemErrors, batchErr := ai.ExecuteBatchContext(ctx, len(eligible), options.MaxWorkers, func(workCtx context.Context, index int) error {
		imgPath := paths[index]
		result, err := ai.ColorizeSingleImageContext(workCtx, imgPath, colorizeOpts)
		if err != nil {
			itemResults[index].err = err
			return err
		}
		itemResults[index].result = result
		if result.Skipped {
			return nil
		}

		resizedBytes, err := docx.ResizeToMatch(imgPath, result.ColorizedPath)
		if err != nil {
			itemResults[index].err = err
			return err
		}
		itemResults[index].resizedBytes = resizedBytes
		return nil
	}, nil)
	for index, err := range itemErrors {
		if itemResults[index].err == nil && err != nil {
			itemResults[index].err = err
		}
	}

	for index, item := range itemResults {
		if item.err != nil {
			img := eligible[index]
			warning := fmt.Sprintf("não foi possível colorir '%s': %v", img.OriginalName, item.err)
			processed.warnings = append(processed.warnings, warning)
			diagnosticf(options, "⚠️ Aviso: %s\n", warning)
			continue
		}
		if item.result == nil {
			continue
		}
		img := eligible[index]
		if item.result.Skipped {
			diagnosticf(options, "⏭️  Pulada pela triagem: %s (%s)\n", img.OriginalName, item.result.SkipReason)
			processed.triageSkipped = append(processed.triageSkipped, ai.TriageSkipInfo{
				Name:   img.OriginalName,
				Stage:  item.result.SkipStage,
				Reason: item.result.SkipReason,
			})
			continue
		}
		processed.replacements[img.PathInZip] = item.resizedBytes
		processed.colorizedResults = append(processed.colorizedResults, *item.result)
	}

	if batchErr != nil {
		return processed, batchErr
	}
	return processed, nil
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

	// 2. Colora cada imagem mantida, redimensiona para a dimensão original e
	// preserva os resultados na ordem do documento.
	processed, err := processExtractedImages(options.Context, tempExtractDir, targetDir, apiKey, model, extractRes.Images, options, triageModel, noTriage)
	if err != nil {
		return nil, err
	}

	// 3. Reconstrói um novo arquivo .docx com as imagens substituídas
	baseName := strings.TrimSuffix(filepath.Base(docxPath), filepath.Ext(docxPath))
	rebuiltDocxName := fmt.Sprintf("%s colorida.docx", baseName)
	rebuiltDocxPath := filepath.Join(targetDir, rebuiltDocxName)

	if len(processed.replacements) > 0 {
		if err := docx.RebuildDocx(docxPath, rebuiltDocxPath, processed.replacements); err != nil {
			return nil, fmt.Errorf("erro ao reconstruir arquivo docx colorida: %w", err)
		}
	}

	return &PipelineResult{
		DocxPath:           docxPath,
		OutputDir:          targetDir,
		RebuiltDocxPath:    rebuiltDocxPath,
		TotalExtracted:     extractRes.TotalExtracted,
		TotalSkipped:       extractRes.TotalSkipped,
		TotalColorized:     len(processed.colorizedResults),
		TotalTriageSkipped: len(processed.triageSkipped),
		TotalFormatSkipped: len(processed.formatSkipped),
		Results:            processed.colorizedResults,
		TriageSkipped:      processed.triageSkipped,
		FormatSkipped:      processed.formatSkipped,
		Warnings:           processed.warnings,
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

	// 2. Colora cada imagem selecionada, redimensiona para a dimensão original
	// e preserva os resultados na ordem recebida.
	processed, err := processExtractedImages(options.Context, tempExtractDir, targetDir, apiKey, model, extractRes.Images, options, triageModel, noTriage)
	if err != nil {
		return nil, err
	}

	// 3. Reconstrói o arquivo .docx com as imagens substituídas
	baseName := strings.TrimSuffix(filepath.Base(docxPath), filepath.Ext(docxPath))
	rebuiltDocxName := fmt.Sprintf("%s colorida.docx", baseName)
	rebuiltDocxPath := filepath.Join(targetDir, rebuiltDocxName)

	if len(processed.replacements) > 0 {
		if err := docx.RebuildDocx(docxPath, rebuiltDocxPath, processed.replacements); err != nil {
			return nil, fmt.Errorf("erro ao reconstruir arquivo docx colorida: %w", err)
		}
	}

	return &PipelineResult{
		DocxPath:           docxPath,
		OutputDir:          targetDir,
		RebuiltDocxPath:    rebuiltDocxPath,
		TotalExtracted:     extractRes.TotalExtracted,
		TotalColorized:     len(processed.colorizedResults),
		TotalTriageSkipped: len(processed.triageSkipped),
		TotalFormatSkipped: len(processed.formatSkipped),
		Results:            processed.colorizedResults,
		TriageSkipped:      processed.triageSkipped,
		FormatSkipped:      processed.formatSkipped,
		Warnings:           processed.warnings,
	}, nil
}

func diagnosticf(options PipelineOptions, format string, args ...interface{}) {
	if !options.Verbose || options.DiagnosticWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(options.DiagnosticWriter, format, args...)
}
