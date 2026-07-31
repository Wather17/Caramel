package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Cores Oficiais do Design System Caramel CLI
var (
	ColorPrimary   = lipgloss.Color("#C02B61") // Magenta Crimson (Brand)
	ColorHighlight = lipgloss.Color("#E8709C") // Vibrant Rose (Item Ativo/Selecionado)
	ColorTag       = lipgloss.Color("#D96B27") // Warm Caramel (Badges/Formatos)
	ColorMuted     = lipgloss.Color("#A19BA8") // Dimmed Gray (Secundário/Instruções)
	ColorWarning   = lipgloss.Color("#E5C07B") // Warm Yellow (Avisos/Alertas)
)

// Estilos Reutilizáveis do Lipgloss
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SelectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorHighlight)

	UnselectedItemStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	TagStyle = lipgloss.NewStyle().
			Foreground(ColorTag).
			Bold(true)

	HintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
)

// GetCaramelTheme retorna um tema customizado do Charm/Huh com a paleta oficial do Caramel
func GetCaramelTheme() *huh.Theme {
	t := huh.ThemeBase()

	// Personalização de Cores do MultiSelect / Forms
	t.Focused.Title = TitleStyle
	t.Focused.SelectedOption = SelectedItemStyle
	t.Focused.UnselectedOption = UnselectedItemStyle
	t.Focused.FocusedButton = lipgloss.NewStyle().Bold(true).Foreground(ColorHighlight).Background(ColorPrimary)
	t.Focused.BlurredButton = lipgloss.NewStyle().Foreground(ColorMuted)
	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(ColorPrimary)
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(ColorHighlight).SetString("> ")

	t.Blurred.Title = lipgloss.NewStyle().Foreground(ColorMuted)
	t.Blurred.SelectedOption = lipgloss.NewStyle().Foreground(ColorMuted)
	t.Blurred.UnselectedOption = lipgloss.NewStyle().Foreground(ColorMuted)

	return t
}
