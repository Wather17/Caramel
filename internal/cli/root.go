package cli

import (
	"fmt"
	"os"

	"caramel/internal/output"

	"github.com/spf13/cobra"
)

var (
	// Version is set during build via ldflags
	Version = "0.3.1-dev"
	Commit  = "none"
	Date    = "unknown"

	verboseFlag bool
	quietFlag   bool
	jsonFlag    bool
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return outputOptions().Validate()
	},
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
	RootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Exibe detalhes de diagnóstico e progresso")
	RootCmd.PersistentFlags().BoolVar(&quietFlag, "quiet", false, "Silencia mensagens de sucesso e progresso")
	RootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Emite o resultado estruturado em JSON")
}

func outputOptions() output.Options {
	return output.Options{Verbose: verboseFlag, Quiet: quietFlag, JSON: jsonFlag}
}
