package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// ShowInteractiveHelp exibe a central de ajuda interativa TUI utilizando Charm/Huh
func ShowInteractiveHelp() error {
	allDocs := GetAllCommandDocs()

	for {
		// 1. Seleção de Categoria
		var selectedCategory string
		catOptions := []huh.Option[string]{
			huh.NewOption("🖼️ Mídia & Arquivos DOCX (process, docx extract)", string(CategoryDocx)),
			huh.NewOption("⚙️ Configurações & IA (setup, show, set)", string(CategoryConfig)),
			huh.NewOption("ℹ️ Sistema & Informações (version, guide)", string(CategorySystem)),
			huh.NewOption("❌ Sair do Guia", "EXIT"),
		}

		formCategory := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("🍬 Central de Ajuda Interativa — Caramel CLI").
					Description("Escolha uma categoria temática para explorar os comandos pedagógicos:").
					Options(catOptions...).
					Value(&selectedCategory),
			),
		).WithTheme(GetCaramelTheme())

		err := formCategory.Run()
		if err != nil || selectedCategory == "EXIT" {
			fmt.Println("👋 Até logo! Até a próxima.")
			return nil
		}

		// 2. Seleção do Comando na Categoria
		var filteredDocs []CommandHelpDoc
		var cmdOptions []huh.Option[string]

		for _, doc := range allDocs {
			if string(doc.Category) == selectedCategory {
				filteredDocs = append(filteredDocs, doc)
				label := fmt.Sprintf("%-22s — %s", doc.Name, doc.Short)
				cmdOptions = append(cmdOptions, huh.NewOption(label, doc.Name))
			}
		}
		cmdOptions = append(cmdOptions, huh.NewOption("↩️ Voltar para Categorias", "BACK"))

		var selectedCmdName string
		formCmd := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("📚 Comandos de %s", selectedCategory)).
					Description("Selecione um comando para ver detalhes e exemplos didáticos:").
					Options(cmdOptions...).
					Value(&selectedCmdName),
			),
		).WithTheme(GetCaramelTheme())

		err = formCmd.Run()
		if err != nil || selectedCmdName == "BACK" {
			continue
		}

		// 3. Exibir Cartão Didático do Comando Selecionado
		var targetDoc *CommandHelpDoc
		for _, d := range filteredDocs {
			if d.Name == selectedCmdName {
				targetDoc = &d
				break
			}
		}

		if targetDoc != nil {
			renderCommandCard(targetDoc)
		}

		// Pergunta se deseja continuar navegando
		var nextAction string
		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("🔍 O que deseja fazer agora?").
					Options(
						huh.NewOption("📚 Ver outro comando", "AGAIN"),
						huh.NewOption("🚪 Sair do Guia", "EXIT"),
					).
					Value(&nextAction),
			),
		).WithTheme(GetCaramelTheme())

		if err := confirmForm.Run(); err != nil || nextAction == "EXIT" {
			fmt.Println("👋 Até logo!")
			return nil
		}
	}
}

// renderCommandCard imprime na tela as informações formatadas do comando
func renderCommandCard(doc *CommandHelpDoc) {
	fmt.Println()
	fmt.Println(HelpHeaderStyle.Render(fmt.Sprintf("📖 Guia Didático: %s", doc.Name)))
	fmt.Println(doc.Short)
	fmt.Println()

	if doc.PedagogicalContext != "" {
		fmt.Println(HelpBoxStyle.Render(doc.PedagogicalContext))
		fmt.Println()
	}

	fmt.Println(HelpSectionTitleStyle.Render("🚀 SINTAXE DE USO:"))
	fmt.Println("  " + HelpSyntaxStyle.Render(doc.Syntax))
	fmt.Println()

	if len(doc.Flags) > 0 {
		fmt.Println(HelpSectionTitleStyle.Render("🚩 FLAGS & OPÇÕES:"))
		for _, f := range doc.Flags {
			fmt.Printf("  %s\n      └─ %s\n", HelpFlagNameStyle.Render(f.Flag), f.Description)
		}
		fmt.Println()
	}

	if len(doc.Examples) > 0 {
		fmt.Println(HelpSectionTitleStyle.Render("💡 EXEMPLOS PRÁTICOS DE USO:"))
		for _, ex := range doc.Examples {
			fmt.Printf("  • %s\n", ex.Description)
			fmt.Printf("    %s\n", HelpSyntaxStyle.Render(ex.Command))
		}
		fmt.Println()
	}
}
