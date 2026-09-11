package cli

import (
	"fmt"
	"strings"

	"caramel/internal/output"
	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

// guideCmd representa o subcomando de guia e ajuda didática, focado em busca
var guideCmd = &cobra.Command{
	Use:     "guide [termo_de_busca]",
	Aliases: []string{"ajuda", "tutorial"},
	Short:   "Guia didático: lista comandos ou busca casos de uso por termo",
	Long: `🍬 Guia Didático do Caramel CLI

Lista todos os comandos disponíveis agrupados por categoria, ou busca por qualquer termo
(ex: 'caramel guide colorir', 'caramel guide caça-palavras', 'caramel guide figma').

A documentação é gerada ao vivo a partir dos próprios comandos — sempre atualizada.

📚 QUANDO USAR:
Use para descobrir comandos por categoria ou encontrar o fluxo adequado a partir de uma
palavra-chave, como 'colorir', 'figma' ou 'caça-palavras'.`,
	Example: `# Listar todos os comandos
caramel guide

# Buscar por termo (palavra-chave, flag ou contexto)
caramel guide triagem
caramel guide 2up`,
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if len(args) > 0 {
			query := strings.Join(args, " ")
			results := ui.SearchCommandDocs(query)
			if renderer.Options().JSON {
				status := output.StateSuccess
				if len(results) == 0 {
					status = output.StateWarning
				}
				return renderer.Result(output.Result{Status: status, Summary: fmt.Sprintf("%d comando(s) encontrado(s).", len(results)), Count: len(results), Data: results})
			}
			return renderer.Text("%s", ui.RenderSearchHelp(query))
		}
		if renderer.Options().JSON {
			docs := ui.GetAllCommandDocs()
			return renderer.Result(output.Result{Status: output.StateSuccess, Summary: fmt.Sprintf("%d comando(s) disponível(is).", len(docs)), Count: len(docs), Data: docs})
		}
		return renderer.Text("%s", ui.RenderGuideOverview())
	},
}

func init() {
	// Alimenta o guia com a árvore de comandos real do Caramel
	ui.SetRootCommand(RootCmd)

	// Customiza a função de ajuda do Cobra:
	// - 'caramel <comando> --help' ou 'caramel help <comando>' → help estilizado do comando
	// - 'caramel help <termo>' (termo não é comando) → busca no guia
	// - 'caramel --help' → help estilizado do root
	RootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Remove tokens de flag de ajuda dos args (ex: '--help', '-h')
		var cleanArgs []string
		for _, a := range args {
			if a != "--help" && a != "-h" && a != "help" {
				cleanArgs = append(cleanArgs, a)
			}
		}

		// Tenta resolver os args como um caminho de comando real
		if len(cleanArgs) > 0 {
			target, _, err := RootCmd.Find(cleanArgs)
			if err == nil && target != RootCmd {
				fmt.Print(ui.RenderStyledHelp(target))
				return
			}
			query := strings.Join(args, " ")
			fmt.Print(ui.RenderSearchHelp(query))
			return
		}

		fmt.Print(ui.RenderStyledHelp(cmd))
	})

	RootCmd.AddCommand(guideCmd)
}
