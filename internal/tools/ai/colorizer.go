package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/prompts"
	"caramel/internal/tools/docx"
)

// ColorizeResult contém o relatório do processo de coloração
type ColorizeResult struct {
	OriginalPath string
	ColorizedPath string
}

// ColorizeSingleImage recebe o caminho de uma imagem, envia para a IA e salva a versão colorida
func ColorizeSingleImage(imagePath string, outputDir string, apiKey string, model string) (*ColorizeResult, error) {
	client, err := NewClient(apiKey)
	if err != nil {
		return nil, err
	}

	prompt := prompts.GetColorizationPrompt()
	imgBytes, ext, err := client.ColorizeImage(imagePath, prompt, model)
	if err != nil {
		return nil, fmt.Errorf("falha ao colorir imagem '%s': %w", filepath.Base(imagePath), err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pasta de saída '%s': %w", outputDir, err)
	}

	baseName := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
	outFileName := fmt.Sprintf("%s_colorida.%s", baseName, ext)
	outputPath := filepath.Join(outputDir, outFileName)

	if err := os.WriteFile(outputPath, imgBytes, 0644); err != nil {
		return nil, fmt.Errorf("falha ao salvar imagem colorida no disco '%s': %w", outputPath, err)
	}

	return &ColorizeResult{
		OriginalPath:  imagePath,
		ColorizedPath: outputPath,
	}, nil
}

// ColorizeDocxImages extrai as imagens de um arquivo .docx e colorida cada uma delas via IA
func ColorizeDocxImages(docxPath string, outputDir string, apiKey string, model string) ([]ColorizeResult, error) {
	// 1. Extrai as imagens originais para uma pasta temporária de trabalho
	tempExtractDir := filepath.Join(outputDir, ".temp_raw_images")
	extractRes, err := docx.ExtractImages(docxPath, tempExtractDir)
	if err != nil {
		return nil, fmt.Errorf("erro na extração inicial de imagens do .docx: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	if extractRes.TotalExtracted == 0 {
		return nil, nil
	}

	var results []ColorizeResult
	for _, img := range extractRes.Images {
		imgPath := filepath.Join(tempExtractDir, img.OriginalName)
		res, err := ColorizeSingleImage(imgPath, outputDir, apiKey, model)
		if err != nil {
			fmt.Printf("⚠️ Aviso: Não foi possível colorir '%s': %v\n", img.OriginalName, err)
			continue
		}
		results = append(results, *res)
	}

	return results, nil
}
