package cli

import (
	"fmt"

	"caramel/internal/output"
	"caramel/internal/tools/docx"

	"github.com/spf13/cobra"
)

var docxMergeOutput string

var docxMergeCmd = &cobra.Command{
	Use:   "merge <arquivo1.docx> <arquivo2.docx> [arquivo3.docx ...]",
	Short: "Junta DOCX preservando ordem, estilos, listas e imagens comuns",
	Long: `Combina dois ou mais arquivos DOCX locais na ordem informada e inicia cada origem em uma nova seção/página.
Preserva texto, tabelas, imagens, estilos, listas e configurações comuns de página. O destino é obrigatório e nunca é sobrescrito.
Comentários, notas de rodapé/fim, revisões controladas e objetos incorporados não são suportados.

📚 QUANDO USAR:
Use para reunir documentos Word em uma apostila ou material único sem enviar os arquivos para um serviço externo.`,
	Example: `# Juntar arquivos na ordem informada
caramel docx merge capa.docx atividades.docx respostas.docx --output apostila.docx

# Usar a forma curta da flag
caramel docx merge parte-1.docx parte-2.docx -o documento-completo.docx`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if err := docx.MergeDOCX(args, docxMergeOutput); err != nil {
			return err
		}
		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("DOCX juntados: %d arquivo(s).", len(args)),
			Count:   len(args),
			Outputs: []string{docxMergeOutput},
		})
	},
}

func init() {
	docxMergeCmd.Flags().StringVarP(&docxMergeOutput, "output", "o", "", "Caminho do DOCX final")
	_ = docxMergeCmd.MarkFlagRequired("output")
	docxCmd.AddCommand(docxMergeCmd)
}
