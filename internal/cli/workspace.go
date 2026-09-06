package cli

import (
	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"studio"},
	Short:   "Abre a área de trabalho visual do Caramel",
	Long: `Abre a área de trabalho interativa do Caramel para navegar pelo vault global,
buscar materiais, importar imagens e encadear geração, coloração e impressão sem
duplicar arquivos por projeto.

📚 QUANDO USAR:
Use quando quiser encontrar materiais reutilizáveis, montar uma coleção temporária
de trabalho e acompanhar operações demoradas.`,
	Example: `# Abrir a área de trabalho visual
caramel workspace`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunWorkspace()
	},
}

func init() {
	RootCmd.AddCommand(workspaceCmd)
}
