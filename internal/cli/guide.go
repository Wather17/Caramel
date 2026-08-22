package cli

import (
	"fmt"
	"strings"

	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var helpInteractive bool

// guideCmd representa o subcomando de guia e ajuda interativa
var guideCmd = &cobra.Command{
	Use:     "guide [termo_de_busca]",
	Aliases: []string{"ajuda", "tutorial"},
	Short:   "Central de Ajuda Interativa e Didática do Caramel CLI",
	Long: `🍬 Central de Ajuda Interativa do Caramel CLI

Navegue visualmente pelo guia didático no terminal ou pesquise diretamente por palavras-chave (ex: figma, papel, 2up, ia).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			query := strings.Join(args, " ")
			fmt.Print(ui.RenderSearchHelp(query))
			return nil
		}
		return ui.ShowInteractiveHelp()
	},
}

func init() {
	// Registra flag -i / --interactive apenas no RootCmd (usada pelo SetHelpFunc)
	RootCmd.Flags().BoolVarP(&helpInteractive, "interactive", "i", false, "Abre a central de ajuda interativa TUI")

	// Customiza a função de ajuda do Cobra para usar o formatador Lipgloss e suportar -i ou busca por argumentos
	RootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		interactive, _ := cmd.Flags().GetBool("interactive")
		if interactive {
			if err := ui.ShowInteractiveHelp(); err != nil {
				fmt.Println("Erro ao exibir ajuda interativa:", err)
			}
			return
		}

		if len(args) > 0 {
			query := strings.Join(args, " ")
			fmt.Print(ui.RenderSearchHelp(query))
			return
		}

		fmt.Print(ui.RenderStyledHelp(cmd))
	})

	RootCmd.AddCommand(guideCmd)
}
