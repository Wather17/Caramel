package cli

import (
	"fmt"

	"caramel/internal/output"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		summary := fmt.Sprintf("Caramel CLI v%s.", Version)
		if Commit != "none" {
			summary += fmt.Sprintf(" Commit: %s.", Commit)
		}
		if Date != "unknown" {
			summary += fmt.Sprintf(" Build: %s.", Date)
		}
		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: summary,
			Data: struct {
				Version string `json:"version"`
				Commit  string `json:"commit,omitempty"`
				Date    string `json:"build_date,omitempty"`
			}{Version: Version, Commit: Commit, Date: Date},
		})
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
