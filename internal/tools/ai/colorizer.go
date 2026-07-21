package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/prompts"
)

// ColorizeResult contém o relatório do processo de coloração
type ColorizeResult struct {
	OriginalPath  string
	ColorizedPath string
}

// ColorizeSingleImage recebe o caminho de uma imagem, envia para a IA e salva a versão colorida
func ColorizeSingleImage(imagePath string, outputDir string, apiKey string, model string, verbose bool) (*ColorizeResult, error) {
	client, err := NewClient(apiKey)
	if err != nil {
		return nil, err
	}
	client.Verbose = verbose

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
