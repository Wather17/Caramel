package ui

import (
	"fmt"
	"strings"

	"caramel/internal/tools/docx"

	"github.com/charmbracelet/huh"
)

// SelectImagesInteractive exibe um menu interativo (TUI) com checkbox para o usuário selecionar as imagens
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
