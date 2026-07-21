package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Exibe a versão atual do Caramel CLI",
	Long:  `Mostra detalhes sobre a versão do Caramel, incluindo hash do commit e data de compilação.`,
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
