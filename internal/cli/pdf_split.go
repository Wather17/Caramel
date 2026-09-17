package cli

import (
	"fmt"

	"caramel/internal/output"
	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var (
	pdfSplitOutputDir string
	pdfSplitRanges    string
)

var pdfSplitCmd = &cobra.Command{
	Use:   "split <arquivo.pdf>",
	Short: "Divide um PDF por página ou por intervalos",
	Long: `Sem --ranges, cria um arquivo PDF para cada página da entrada. Com --ranges, cria um arquivo
por página ou intervalo informado, usando índices inclusivos iniciados em 1. Intervalos inválidos
são rejeitados antes de qualquer arquivo de saída ser criado.

Por padrão, os arquivos são gravados em uma pasta <nome_do_pdf>_split ao lado da entrada. Use
--output-dir para escolher outra pasta. O PDF original não é alterado.

📚 QUANDO USAR:
Use para separar páginas de um material ou salvar capítulos selecionados em PDFs independentes.
A operação acontece localmente e mantém a ordem original dos intervalos informados.`,
	Example: `# Criar um PDF por página na pasta documento_split
caramel pdf split documento.pdf

# Criar um arquivo para cada intervalo informado
caramel pdf split apostila.pdf --ranges 1-3,4-6

# Selecionar intervalos e páginas isoladas em outra pasta
caramel pdf split apostila.pdf --ranges 1-3,5,7-9 --output-dir ./capitulos`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		var rangeSpec *string
		if cmd.Flags().Changed("ranges") {
			rangeSpec = &pdfSplitRanges
		}
		outputPaths, err := pdf.SplitPDF(args[0], pdfSplitOutputDir, rangeSpec)
		if err != nil {
			return err
		}

		renderer.Diagnostic("gerados %d arquivo(s) PDF a partir de %s\n", len(outputPaths), args[0])
		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("PDF dividido: %d arquivo(s) gerado(s).", len(outputPaths)),
			Count:   len(outputPaths),
			Outputs: outputPaths,
		})
	},
}

func init() {
	pdfSplitCmd.Flags().StringVar(&pdfSplitRanges, "ranges", "", "Páginas/intervalos inclusivos, como 1-3,5,7-9")
	pdfSplitCmd.Flags().StringVarP(&pdfSplitOutputDir, "output-dir", "o", "", "Pasta para os PDFs gerados")
	pdfCmd.AddCommand(pdfSplitCmd)
}
