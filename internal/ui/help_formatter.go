package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Estilos de Ajuda baseados nas cores oficiais do Caramel CLI
var (
	HelpHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginTop(1).
			MarginBottom(1)

	HelpSectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorTag)

	HelpCommandNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorHighlight)

	HelpFlagNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorHighlight)

	HelpSyntaxStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#2E2B38")).
			Padding(0, 1)

	HelpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)
)

// RenderStyledHelp customiza a saída de ajuda de um comando Cobra utilizando Lipgloss
func RenderStyledHelp(cmd *cobra.Command) string {
	var sb strings.Builder

	// 🍬 Cabeçalho Principal
	sb.WriteString(HelpHeaderStyle.Render("🍬 Caramel CLI — " + cmd.Name()))
	sb.WriteString("\n")

	// Descrição Longa ou Curta
	desc := cmd.Long
	if desc == "" {
		desc = cmd.Short
	}
	if desc != "" {
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}

	// 💡 Busca na central de documentação se houver dados didáticos para este comando
	doc := findDocForCommand(cmd.CommandPath())
	if doc != nil && doc.PedagogicalContext != "" {
		boxContent := fmt.Sprintf("%s\n\n%s",
			HelpSectionTitleStyle.Render("📚 DICA PEDAGÓGICA & USO"),
			doc.PedagogicalContext,
		)
		sb.WriteString(HelpBoxStyle.Render(boxContent))
		sb.WriteString("\n\n")
	}

	// 🚀 Sintaxe de Uso
	if cmd.Runnable() {
		sb.WriteString(HelpSectionTitleStyle.Render("🚀 SINTAXE DE USO:"))
		sb.WriteString("\n  ")
		sb.WriteString(HelpSyntaxStyle.Render(cmd.UseLine()))
		sb.WriteString("\n\n")
	}

	// 📋 Subcomandos Disponíveis (se houver)
	if cmd.HasAvailableSubCommands() {
		sb.WriteString(HelpSectionTitleStyle.Render("📂 SUBCOMANDOS DISPONÍVEIS:"))
		sb.WriteString("\n")
		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() {
				sb.WriteString(fmt.Sprintf("  %s %s\n",
					HelpCommandNameStyle.Render(fmt.Sprintf("%-16s", sub.Name())),
					sub.Short,
				))
			}
		}
		sb.WriteString("\n")
	}

	// 💡 Exemplos Práticos
	if doc != nil && len(doc.Examples) > 0 {
		sb.WriteString(HelpSectionTitleStyle.Render("💡 EXEMPLOS PRÁTICOS:"))
		sb.WriteString("\n")
		for _, ex := range doc.Examples {
			sb.WriteString(fmt.Sprintf("  • %s\n", ex.Description))
			sb.WriteString(fmt.Sprintf("    %s\n", HelpSyntaxStyle.Render(ex.Command)))
		}
		sb.WriteString("\n")
	} else if cmd.Example != "" {
		sb.WriteString(HelpSectionTitleStyle.Render("💡 EXEMPLOS PRÁTICOS:"))
		sb.WriteString("\n  ")
		sb.WriteString(cmd.Example)
		sb.WriteString("\n\n")
	}

	// 🚩 Flags / Opções
	if cmd.HasAvailableLocalFlags() {
		sb.WriteString(HelpSectionTitleStyle.Render("🚩 OPÇÕES & FLAGS:"))
		sb.WriteString("\n")
		sb.WriteString(cmd.LocalFlags().FlagUsages())
		sb.WriteString("\n")
	}

	// Dica do modo interativo
	sb.WriteString(HintStyle.Render("💡 DICA: Use 'caramel guide' ou 'caramel help -i' para abrir a central de ajuda interativa TUI."))
	sb.WriteString("\n")

	return sb.String()
}

// findDocForCommand busca o CommandHelpDoc pelo caminho do comando ou pelo nome base
func findDocForCommand(cmdPath string) *CommandHelpDoc {
	docs := GetAllCommandDocs()
	for _, d := range docs {
		cleanDName := strings.TrimPrefix(d.Name, "caramel ")
		cleanCmdPath := strings.TrimPrefix(cmdPath, "caramel ")
		if d.Name == cmdPath || strings.HasSuffix(cleanCmdPath, cleanDName) || strings.HasSuffix(cleanDName, cleanCmdPath) {
			return &d
		}
	}
	return nil
}
