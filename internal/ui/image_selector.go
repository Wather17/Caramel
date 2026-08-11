package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/tools/docx"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// SelectImagesInteractive exibe um menu interativo (TUI) com checkbox para o usuário selecionar as imagens do .docx
func SelectImagesInteractive(images []docx.ExtractedImage) ([]docx.ExtractedImage, error) {
	if len(images) == 0 {
		return nil, nil
	}

	var selectedNames []string
	options := make([]huh.Option[string], 0, len(images))

	for _, img := range images {
		sizeKB := float64(img.Size) / 1024.0
		label := fmt.Sprintf("%-20s  [%-4s]  %6.1f KB", img.OriginalName, strings.ToUpper(img.Format), sizeKB)

		// Por padrão, todas as imagens vem selecionadas
		options = append(options, huh.NewOption(label, img.OriginalName).Selected(true))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("🍬 Seleção Interativa de Imagens").
				Description("Use as setas [↑/↓] para navegar, [Espaço] para marcar/desmarcar e [Enter] para confirmar.").
				Options(options...).
				Value(&selectedNames),
		),
	).WithTheme(GetCaramelTheme())

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return nil, fmt.Errorf("operação cancelada pelo usuário")
		}
		return nil, fmt.Errorf("falha ao executar menu interativo: %w", err)
	}

	selectedMap := make(map[string]bool, len(selectedNames))
	for _, name := range selectedNames {
		selectedMap[name] = true
	}

	filtered := make([]docx.ExtractedImage, 0, len(selectedNames))
	for _, img := range images {
		if selectedMap[img.OriginalName] {
			filtered = append(filtered, img)
		}
	}

	return filtered, nil
}

// SelectImageFilesWithPreviewInteractive exibe um menu TUI interativo com galeria de miniaturas ANSI no terminal
// permitindo ao usuário visualizar e marcar quais arquivos de imagem deseja processar.
func SelectImageFilesWithPreviewInteractive(imagePaths []string) ([]string, error) {
	if len(imagePaths) == 0 {
		return nil, nil
	}

	// Se houver apenas 1 imagem, seleciona-a diretamente sem necessitar de formulário
	if len(imagePaths) == 1 {
		return imagePaths, nil
	}

	fmt.Println(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF8C00")).
		Bold(true).
		Render("\n🎨 Galeria de Previews (Terminal ANSI TrueColor):"))

	options := make([]huh.Option[string], 0, len(imagePaths))

	for i, path := range imagePaths {
		baseName := filepath.Base(path)
		info, err := os.Stat(path)
		sizeKB := 0.0
		if err == nil {
			sizeKB = float64(info.Size()) / 1024.0
		}

		// Renderiza miniatura ANSI (ex: 22 colunas por 6 linhas)
		ansiPreview, err := RenderImageFileToANSI(path, 22, 6)
		if err == nil && ansiPreview != "" {
			fmt.Printf("\n--- [%d] %s (%.1f KB) ---\n%s", i+1, baseName, sizeKB, ansiPreview)
		}

		label := fmt.Sprintf("%-24s  (%.1f KB)", baseName, sizeKB)
		options = append(options, huh.NewOption(label, path).Selected(true))
	}

	var selectedPaths []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("🍬 Seleção de Imagens para Coloração").
				Description("Verifique as miniaturas acima. Use [↑/↓] para navegar, [Espaço] para selecionar/desmarcar e [Enter] para confirmar.").
				Options(options...).
				Value(&selectedPaths),
		),
	).WithTheme(GetCaramelTheme())

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return nil, fmt.Errorf("operação cancelada pelo usuário")
		}
		return nil, fmt.Errorf("falha ao executar formulário de seleção de imagens: %w", err)
	}

	return selectedPaths, nil
}
