package cli

import (
	"fmt"

	"caramel/internal/output"
	"caramel/internal/tools/docx"

	"github.com/spf13/cobra"
)

var docxSplitOutputDir string

var docxSplitCmd = &cobra.Command{
	Use:   "split <arquivo.docx>",
	Short: "Divide um DOCX por quebras explícitas de página ou seção",
	Long: `Cria um arquivo DOCX por segmento separado por uma quebra manual de página ou por uma quebra de seção.
DOCX não possui paginação fixa: o comando não divide por páginas visuais nem usa quebras renderizadas pelo Word.

Por padrão, as partes são gravadas em uma pasta <nome>_split ao lado do documento, com nomes sequenciais.
Use --output-dir para escolher outra pasta. O arquivo original não é alterado e saídas existentes não são substituídas.
As quebras precisam estar em parágrafos de nível superior; quebras dentro de tabelas ou blocos aninhados retornam erro.

📚 QUANDO USAR:
Use para separar um material que já contenha quebras de página ou de seção inseridas no documento.`,
	Example: `# Gerar uma parte para cada quebra explícita
caramel docx split apostila.docx

# Escolher a pasta de destino
caramel docx split apostila.docx --output-dir ./capitulos`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		paths, err := docx.SplitDOCX(args[0], docxSplitOutputDir)
		if err != nil {
			return err
		}
		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("DOCX dividido: %d arquivo(s) gerado(s).", len(paths)),
			Count:   len(paths),
			Outputs: paths,
		})
	},
}

func init() {
	docxSplitCmd.Flags().StringVarP(&docxSplitOutputDir, "output-dir", "o", "", "Pasta para os DOCX gerados")
	docxCmd.AddCommand(docxSplitCmd)
}
