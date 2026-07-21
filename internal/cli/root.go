package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set during build via ldflags
	Version = "0.1.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "caramel",
	Short: "Caramel é um CLI com ferramentas utilitárias para desenvolvimento pedagógico",
	Long: `🍬 Caramel CLI

Uma suíte de ferramentas de linha de comando projetada para auxiliar no uso diário,
criação de atividades e utilitários de desenvolvimento pedagógico.

Para mais informações sobre os comandos disponíveis, use:
  caramel --help`,
	// Uncomment the following line if your bare application has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao executar o comando: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Root flags can be defined here if needed
}
