package cli

import (
	"github.com/spf13/cobra"
)

// printCmd representa o grupo de comandos de preparação para impressão
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Preparação de materiais para impressão (2 por folha, fichas)",
	Long:  `Comandos para diagramar e gerar PDFs de impressão: 2-up (duas atividades por folha) e fichas pedagógicas A4.`,
}