package cli

import (
	"fmt"
	"strings"

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

A documentação é gerada ao vivo a partir dos próprios comandos — sempre atualizada.`,
	Example: `# Listar todos os comandos
caramel guide

# Buscar por termo (palavra-chave, flag ou contexto)
caramel guide triagem
caramel guide 2up`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			query := strings.Join(args, " ")
			fmt.Print(ui.RenderSearchHelp(query))
			return nil
		}
		fmt.Print(ui.RenderGuideOverview())
		return nil
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
			if err == nil && target != RootCmd && target.Runnable() {
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