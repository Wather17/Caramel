package cli

import (
	"fmt"
	"strings"

	"caramel/internal/output"
	"caramel/internal/tools/docx"

	"github.com/spf13/cobra"
)

var docxMergeOutput string

var docxMergeCmd = &cobra.Command{
	Use:   "merge <arquivo1.docx> <arquivo2.docx> [arquivo3.docx ...]",
	Short: "Junta DOCX preservando ordem, estilos, listas e imagens comuns",
	Long: `Combina dois ou mais arquivos DOCX locais na ordem informada e inicia cada origem em uma nova seção/página.
Preserva texto, tabelas, imagens, estilos, listas e configurações comuns de página. Sem biblioteca configurada, o destino é obrigatório; resultados nunca são sobrescritos.
Comentários, notas de rodapé/fim, revisões controladas e objetos incorporados não são suportados.

📚 QUANDO USAR:
Use para reunir documentos Word em uma apostila ou material único sem enviar os arquivos para um serviço externo.`,
	Example: `# Juntar arquivos na ordem informada
caramel docx merge capa.docx atividades.docx respostas.docx --output apostila.docx

# Usar a forma curta da flag
caramel docx merge parte-1.docx parte-2.docx -o documento-completo.docx`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptionsFor(cmd), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		targetPath := docxMergeOutput
		var plan *output.Plan
		if !cmd.Flags().Changed("output") || strings.TrimSpace(targetPath) == "" {
			plan, err = libraryOutputPlan(cmd, "docx", output.ShapeSingle, outputStem(args[0], "_merged"), ".docx")
			if err != nil {
				return err
			}
			if plan == nil {
				return fmt.Errorf("a flag --output é obrigatória quando a biblioteca não está configurada")
			}
			defer plan.Cleanup()
			targetPath = plan.WorkPath()
		}
		if err := docx.MergeDOCX(args, targetPath); err != nil {
			return err
		}
		paths, err := publishOutputPlan(plan, []string{targetPath})
		if err != nil {
			return err
		}
		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("DOCX juntados: %d arquivo(s).", len(args)),
			Count:   len(args),
			Outputs: paths,
		})
	},
}

func init() {
	docxMergeCmd.Flags().StringVarP(&docxMergeOutput, "output", "o", "", "Caminho do DOCX final")
	docxCmd.AddCommand(docxMergeCmd)
}
