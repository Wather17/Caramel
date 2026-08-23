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

// RenderStyledHelp customiza a saída de ajuda de um comando Cobra utilizando Lipgloss.
// Todo o conteúdo (descrição, contexto pedagógico no Long, sintaxe, exemplos e flags)
// vem diretamente do próprio comando Cobra — sem listas manuais.
func RenderStyledHelp(cmd *cobra.Command) string {
	var sb strings.Builder

	sb.WriteString(HelpHeaderStyle.Render("🍬 Caramel CLI — " + cmd.Name()))
	sb.WriteString("\n")

	desc := cmd.Long
	if desc == "" {
		desc = cmd.Short
	}
	if desc != "" {
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}

	if cmd.Runnable() {
		sb.WriteString(HelpSectionTitleStyle.Render("🚀 SINTAXE DE USO:"))
		sb.WriteString("\n  ")
		sb.WriteString(HelpSyntaxStyle.Render(cmd.UseLine()))
		sb.WriteString("\n\n")
	}

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

	if cmd.Example != "" {
		sb.WriteString(HelpSectionTitleStyle.Render("💡 EXEMPLOS PRÁTICOS:"))
		sb.WriteString("\n")
		sb.WriteString(cmd.Example)
		sb.WriteString("\n\n")
	}

	if cmd.HasAvailableLocalFlags() {
		sb.WriteString(HelpSectionTitleStyle.Render("🚩 OPÇÕES & FLAGS:"))
		sb.WriteString("\n")
		sb.WriteString(cmd.LocalFlags().FlagUsages())
		sb.WriteString("\n")
	}

	sb.WriteString(HintStyle.Render("💡 DICA: Use 'caramel guide' para listar os comandos ou 'caramel guide <termo>' para buscar."))
	sb.WriteString("\n")

	return sb.String()
}

// SearchCommandDocs realiza uma busca em texto livre (full-text) nos documentos ao vivo:
// nome, resumo, contexto pedagógico (Long), sintaxe, aliases, flags e exemplos
func SearchCommandDocs(query string) []CommandHelpDoc {
	cleanQuery := strings.ToLower(strings.TrimSpace(query))
	if cleanQuery == "" {
		return nil
	}

	var results []CommandHelpDoc
	for _, doc := range GetAllCommandDocs() {
		haystack := strings.ToLower(doc.Name + " " + doc.Short + " " + doc.PedagogicalContext +
			" " + doc.Syntax + " " + doc.Aliases)

		for _, f := range doc.Flags {
			haystack += " " + f.Flag + " " + f.Description
		}
		for _, ex := range doc.Examples {
			haystack += " " + ex.Description + " " + ex.Command
		}

		if strings.Contains(haystack, cleanQuery) {
			results = append(results, doc)
		}
	}
	return results
}

// RenderSearchHelp renderiza o resultado da busca por palavras-chave pedagógicas no terminal
func RenderSearchHelp(query string) string {
	var sb strings.Builder
	results := SearchCommandDocs(query)

	sb.WriteString(HelpHeaderStyle.Render(fmt.Sprintf("🔍 Guia Didático de Ajuda — Busca por: '%s'", query)))
	sb.WriteString("\n\n")

	if len(results) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Render(
			fmt.Sprintf("Nenhum comando ou caso de uso encontrado para o termo '%s'.\n", query),
		))
		sb.WriteString(HintStyle.Render("\n💡 DICA: Digite 'caramel guide' para listar todos os comandos."))
		sb.WriteString("\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Encontrado(s) %d resultado(s) relevante(s):\n\n", len(results)))

	for _, doc := range results {
		boxContent := fmt.Sprintf("%s %s\n%s\n\n%s\n\n%s %s",
			HelpSectionTitleStyle.Render("📌 COMANDO:"),
			HelpCommandNameStyle.Render(doc.Name),
			doc.Short,
			doc.PedagogicalContext,
			HelpSectionTitleStyle.Render("🚀 SINTAXE:"),
			HelpSyntaxStyle.Render(doc.Syntax),
		)

		if len(doc.Examples) > 0 {
			boxContent += fmt.Sprintf("\n\n%s\n", HelpSectionTitleStyle.Render("💡 EXEMPLOS PRÁTICOS:"))
			for _, ex := range doc.Examples {
				boxContent += fmt.Sprintf("  • %s\n    %s\n", ex.Description, HelpSyntaxStyle.Render(ex.Command))
			}
		}

		sb.WriteString(HelpBoxStyle.Render(boxContent))
		sb.WriteString("\n\n")
	}

	sb.WriteString(HintStyle.Render("💡 DICA: Use 'caramel guide <termo>' para refinar a busca, ou 'caramel <comando> --help' para detalhes."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderGuideOverview exibe a lista completa de comandos agrupados por categoria,
// derivada ao vivo da árvore Cobra — o guia é focado em busca e navegação simples.
func RenderGuideOverview() string {
	var sb strings.Builder

	sb.WriteString(HelpHeaderStyle.Render("🍬 Guia Didático — Caramel CLI"))
	sb.WriteString("\n")
	sb.WriteString("Lista de todos os comandos disponíveis, agrupados por categoria.\n")
	sb.WriteString(HintStyle.Render("💡 DICA: Use 'caramel guide <termo>' para buscar (ex: 'caramel guide caça-palavras')."))
	sb.WriteString("\n\n")

	categories := []struct {
		label string
		docs  []CommandHelpDoc
	}{
		{string(CategoryDocx), nil},
		{string(CategoryImage), nil},
		{string(CategoryPrint), nil},
		{string(CategoryRoutine), nil},
		{string(CategoryConfig), nil},
		{string(CategorySystem), nil},
	}

	for _, doc := range GetAllCommandDocs() {
		for i := range categories {
			if categories[i].label == string(doc.Category) {
				categories[i].docs = append(categories[i].docs, doc)
			}
		}
	}

	for _, cat := range categories {
		if len(cat.docs) == 0 {
			continue
		}
		sb.WriteString(HelpSectionTitleStyle.Render(cat.label))
		sb.WriteString("\n")
		for _, doc := range cat.docs {
			line := fmt.Sprintf("  %s %s\n",
				HelpCommandNameStyle.Render(fmt.Sprintf("%-24s", doc.Name)),
				doc.Short,
			)
			if doc.Aliases != "" {
				line += fmt.Sprintf("     └─ aliases: %s\n", doc.Aliases)
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	sb.WriteString(HintStyle.Render("💡 DICA: 'caramel <comando> --help' mostra detalhes completos de qualquer comando."))
	sb.WriteString("\n")

	return sb.String()
}