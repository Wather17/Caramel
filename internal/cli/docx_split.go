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

Com a biblioteca configurada, as partes ficam no diário de resultados; sem ela, são gravadas em
uma pasta <nome>_split ao lado do documento. Use --output-dir para escolher outra pasta. O arquivo
original não é alterado e saídas existentes não são substituídas.
As quebras precisam estar em parágrafos de nível superior; quebras dentro de tabelas ou blocos aninhados retornam erro.

📚 QUANDO USAR:
Use para separar um material que já contenha quebras de página ou de seção inseridas no documento.`,
	Example: `# Gerar uma parte para cada quebra explícita
caramel docx split apostila.docx

# Escolher a pasta de destino
caramel docx split apostila.docx --output-dir ./capitulos`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptionsFor(cmd), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		targetDir := docxSplitOutputDir
		var plan *output.Plan
		if !cmd.Flags().Changed("output-dir") {
			plan, err = libraryOutputPlan(cmd, "docx", output.ShapeBundle, outputStem(args[0], "_split"), "")
			if err != nil {
				return err
			}
			if plan != nil {
				defer plan.Cleanup()
				targetDir = plan.WorkPath()
			}
		}

		paths, err := docx.SplitDOCX(args[0], targetDir)
		if err != nil {
			return err
		}
		paths, err = publishOutputPlan(plan, paths)
		if err != nil {
			return err
		}
		return renderer.Result(output.Result{
			Status:        output.StateSuccess,
			Summary:       fmt.Sprintf("DOCX dividido: %d arquivo(s) gerado(s).", len(paths)),
			Count:         len(paths),
			Outputs:       paths,
			PrimaryOutput: bundlePrimaryOutput(plan, paths),
		})
	},
}

func init() {
	docxSplitCmd.Flags().StringVarP(&docxSplitOutputDir, "output-dir", "o", "", "Pasta para os DOCX gerados")
	docxCmd.AddCommand(docxSplitCmd)
}
