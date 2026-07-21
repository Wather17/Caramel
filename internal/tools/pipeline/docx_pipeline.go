package pipeline

import (
	"fmt"
	"os"
	"path/filepath"

	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
)

// PipelineResult guarda o resumo da execução do pipeline automatizado
type PipelineResult struct {
	DocxPath       string
	OutputDir      string
	TotalExtracted int
	TotalColorized int
	Results        []ai.ColorizeResult
}

// RunDocxPipeline executa o fluxo completo:
// 1. Extração automática de todas as imagens contidas no .docx
// 2. Sanitização automática do nome do diretório de destino
// 3. Coloração de cada imagem via IA (OpenRouter / Nano Banana 2)
func RunDocxPipeline(docxPath string, outputDir string, apiKey string, model string, verbose bool) (*PipelineResult, error) {
	targetDir := outputDir
	if targetDir == "" {
		targetDir = docx.SanitizeFolderName(docxPath)
	}

	// 1. Extrai as imagens originais para uma pasta temporária de trabalho
	tempExtractDir := filepath.Join(targetDir, ".temp_raw_images")
	extractRes, err := docx.ExtractImages(docxPath, tempExtractDir)
	if err != nil {
		return nil, fmt.Errorf("erro na extração inicial de imagens do .docx: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	if extractRes.TotalExtracted == 0 {
		return &PipelineResult{
			DocxPath:  docxPath,
			OutputDir: targetDir,
		}, nil
	}

	// 2. Colora cada imagem extraída
	var colorizedResults []ai.ColorizeResult
	for _, img := range extractRes.Images {
		imgPath := filepath.Join(tempExtractDir, img.OriginalName)
		res, err := ai.ColorizeSingleImage(imgPath, targetDir, apiKey, model, verbose)
		if err != nil {
			fmt.Printf("⚠️ Aviso: Não foi possível colorir '%s': %v\n", img.OriginalName, err)
			continue
		}
		colorizedResults = append(colorizedResults, *res)
	}

	return &PipelineResult{
		DocxPath:       docxPath,
		OutputDir:      targetDir,
		TotalExtracted: extractRes.TotalExtracted,
		TotalColorized: len(colorizedResults),
		Results:        colorizedResults,
	}, nil
}
