package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
)

// PipelineResult guarda o resumo da execução do pipeline automatizado
type PipelineResult struct {
	DocxPath         string
	OutputDir        string
	RebuiltDocxPath  string
	TotalExtracted   int
	TotalSkipped     int
	TotalColorized   int
	Results          []ai.ColorizeResult
	SkippedImages    []docx.ExtractedImage
}

// RunDocxPipeline executa o fluxo completo:
// 1. Extração automática de todas as imagens contidas no .docx (filtrando por tamanho mínimo)
// 2. Sanitização automática do nome do diretório de destino
// 3. Coloração de cada imagem via IA (OpenRouter / Nano Banana 2)
// 4. Redimensionamento para o tamanho original das imagens
// 5. Reconstrução de um novo arquivo .docx com as imagens substituídas
func RunDocxPipeline(docxPath string, outputDir string, apiKey string, model string, minSizeBytes int64, verbose bool) (*PipelineResult, error) {
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
	var colorizedResults []ai.ColorizeResult
	replacements := make(map[string][]byte)

	for _, img := range extractRes.Images {
		imgPath := filepath.Join(tempExtractDir, img.OriginalName)
		res, err := ai.ColorizeSingleImage(imgPath, targetDir, apiKey, model, verbose)
		if err != nil {
			fmt.Printf("⚠️ Aviso: Não foi possível colorir '%s': %v\n", img.OriginalName, err)
			continue
		}

		// Redimensiona a imagem gerada para ter exatamente os mesmos pixels da original
		resizedBytes, err := docx.ResizeToMatch(imgPath, res.ColorizedPath)
		if err != nil {
			fmt.Printf("⚠️ Aviso: Falha ao ajustar tamanho da imagem colorida '%s': %v\n", img.OriginalName, err)
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
		DocxPath:         docxPath,
		OutputDir:        targetDir,
		RebuiltDocxPath:  rebuiltDocxPath,
		TotalExtracted:   extractRes.TotalExtracted,
		TotalSkipped:     extractRes.TotalSkipped,
		TotalColorized:   len(colorizedResults),
		Results:          colorizedResults,
		SkippedImages:    extractRes.SkippedImages,
	}, nil
}

// RunDocxPipelineSelected executa o pipeline apenas nas imagens pré-selecionadas pelo usuário
func RunDocxPipelineSelected(docxPath string, outputDir string, apiKey string, model string, selectedImages []docx.ExtractedImage, verbose bool) (*PipelineResult, error) {
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
	var colorizedResults []ai.ColorizeResult
	replacements := make(map[string][]byte)

	for _, img := range extractRes.Images {
		imgPath := filepath.Join(tempExtractDir, img.OriginalName)
		res, err := ai.ColorizeSingleImage(imgPath, targetDir, apiKey, model, verbose)
		if err != nil {
			fmt.Printf("⚠️ Aviso: Não foi possível colorir '%s': %v\n", img.OriginalName, err)
			continue
		}

		// Redimensiona a imagem gerada para ter exatamente os mesmos pixels da original
		resizedBytes, err := docx.ResizeToMatch(imgPath, res.ColorizedPath)
		if err != nil {
			fmt.Printf("⚠️ Aviso: Falha ao ajustar tamanho da imagem colorida '%s': %v\n", img.OriginalName, err)
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
		DocxPath:        docxPath,
		OutputDir:       targetDir,
		RebuiltDocxPath: rebuiltDocxPath,
		TotalExtracted:  extractRes.TotalExtracted,
		TotalColorized:  len(colorizedResults),
		Results:         colorizedResults,
	}, nil
}
