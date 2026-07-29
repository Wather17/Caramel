package cli

import (
	"fmt"

	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var helpInteractive bool

// guideCmd representa o subcomando de guia e ajuda interativa
var guideCmd = &cobra.Command{
	Use:     "guide",
	Aliases: []string{"ajuda", "tutorial"},
	Short:   "Central de Ajuda Interativa e Didática do Caramel CLI",
	Long: `🍬 Central de Ajuda Interativa do Caramel CLI

Navegue visualmente pelo guia didático no terminal. Veja detalhes de cada comando,
casos de uso pedagógico, explicações de flags e exemplos copiáveis.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.ShowInteractiveHelp()
	},
}

func init() {
	// Registra flag -i / --interactive no comando de ajuda padrão e no guide
	guideCmd.Flags().BoolVarP(&helpInteractive, "interactive", "i", false, "Abre a central de ajuda interativa TUI")
	RootCmd.Flags().BoolVarP(&helpInteractive, "interactive", "i", false, "Abre a central de ajuda interativa TUI")

	// Customiza a função de ajuda do Cobra para usar o formatador Lipgloss e suportar -i
	RootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		interactive, _ := cmd.Flags().GetBool("interactive")
		if interactive {
			if err := ui.ShowInteractiveHelp(); err != nil {
				fmt.Println("Erro ao exibir ajuda interativa:", err)
			}
			return
		}
		fmt.Print(ui.RenderStyledHelp(cmd))
	})

	RootCmd.AddCommand(guideCmd)
}
