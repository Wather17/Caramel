package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Exibe a versão atual do Caramel CLI",
	Long: `Mostra detalhes sobre a versão do Caramel, incluindo hash do commit e data de compilação.

📚 QUANDO USAR:
Use para verificar qual versão do Caramel está instalada — útil para conferir se sua instalação
está atualizada em relação às releases do GitHub.`,
	Example: `# Exibir informações de versão
caramel version`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("🍬 Caramel CLI v%s\n", Version)
		if Commit != "none" {
			fmt.Printf("   Commit: %s\n", Commit)
		}
		if Date != "unknown" {
			fmt.Printf("   Data de Build: %s\n", Date)
		}
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
