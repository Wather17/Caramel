package cli

import (
	"fmt"

	"caramel/internal/output"
	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var pdfMergeOutput string

var pdfMergeCmd = &cobra.Command{
	Use:   "merge <arquivo1.pdf> <arquivo2.pdf> [arquivo3.pdf ...]",
	Short: "Junta PDFs preservando a ordem e o tamanho das páginas",
	Long: `Combina dois ou mais PDFs locais em um único arquivo. As páginas seguem primeiro a ordem dos
argumentos e depois a ordem original de cada PDF. O tamanho e a orientação de cada página são
preservados. Arquivos protegidos por senha não são aceitos.

📚 QUANDO USAR:
Use para reunir atividades, avaliações ou capítulos em uma apostila. A operação acontece
localmente e não envia seus arquivos para um serviço externo.`,
	Example: `# Juntar arquivos na ordem informada
caramel pdf merge capa.pdf atividades.pdf respostas.pdf --output apostila.pdf

# Especificar o destino com a forma curta da flag
caramel pdf merge parte-1.pdf parte-2.pdf -o documento-completo.pdf`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		renderer.Diagnostic("juntando %d arquivos PDF\n", len(args))
		pageCount, err := pdf.MergePDFs(args, pdfMergeOutput)
		if err != nil {
			return err
		}

		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("PDFs juntados: %d arquivo(s), %d página(s).", len(args), pageCount),
			Count:   pageCount,
			Outputs: []string{pdfMergeOutput},
		})
	},
}

var pdfCmd = &cobra.Command{
	Use:   "pdf",
	Short: "Operações com arquivos PDF",
	Long:  "Ferramentas locais para criar, juntar, dividir, renderizar e extrair conteúdo de arquivos PDF.",
}

func init() {
	pdfMergeCmd.Flags().StringVarP(&pdfMergeOutput, "output", "o", "", "Caminho do PDF final")
	_ = pdfMergeCmd.MarkFlagRequired("output")
	pdfCmd.AddCommand(pdfMergeCmd)
	RootCmd.AddCommand(pdfCmd)
}
