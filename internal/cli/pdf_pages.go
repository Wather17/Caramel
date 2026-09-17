package cli

import (
	"fmt"

	"caramel/internal/output"
	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var (
	pdfPagesFormat    string
	pdfPagesDPI       int
	pdfPagesOutputDir string
)

var pdfPagesRenderCmd = &cobra.Command{
	Use:   "render <entrada.pdf>",
	Short: "Exporta páginas completas de PDF como imagens",
	Long: "Renderiza todas as páginas, incluindo texto, desenhos vetoriais e imagens, como arquivos raster\n" +
		"numerados na ordem original. O padrão é PNG em 150 DPI; --format aceita png ou jpg e --dpi\n" +
		"permite escolher outra resolução positiva até 1200 DPI, respeitando um limite de pixels por página.\n\n" +
		"Por padrão, as imagens são salvas na pasta <nome_do_pdf>_rendered_pages ao lado da entrada. Use\n" +
		"--output-dir para escolher outra pasta. A renderização é local e usa um backend WebAssembly sem\n" +
		"dependências de sistema.\n\n" +
		"📚 QUANDO USAR:\n" +
		"Use quando precisar da página visual completa para compartilhar, visualizar ou reutilizar em outro\n" +
		"fluxo. Para recuperar os arquivos de imagem originais embutidos, use pdf images extract.",
	Example: "# Exportar todas as páginas como PNG a 150 DPI\n" +
		"caramel pdf pages render apostila.pdf\n\n" +
		"# Exportar como JPEG a 300 DPI em uma pasta escolhida\n" +
		"caramel pdf pages render apostila.pdf --format jpg --dpi 300 --output-dir ./paginas",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		outputPaths, err := pdf.RenderPDFPages(args[0], pdfPagesOutputDir, pdfPagesFormat, pdfPagesDPI)
		if err != nil {
			return err
		}
		renderer.Diagnostic("renderizadas %d páginas de %s\n", len(outputPaths), args[0])
		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("Páginas renderizadas: %d imagem(ns).", len(outputPaths)),
			Count:   len(outputPaths),
			Outputs: outputPaths,
		})
	},
}

var pdfPagesCmd = &cobra.Command{
	Use:   "pages",
	Short: "Renderização e extração por página",
	Long:  "Operações que trabalham com páginas completas de arquivos PDF.",
}

func init() {
	pdfPagesRenderCmd.Flags().StringVar(&pdfPagesFormat, "format", "png", "Formato das imagens: png ou jpg")
	pdfPagesRenderCmd.Flags().IntVar(&pdfPagesDPI, "dpi", 150, "Resolução por página, entre 1 e 1200 DPI, sujeita ao limite de pixels")
	pdfPagesRenderCmd.Flags().StringVarP(&pdfPagesOutputDir, "output-dir", "o", "", "Pasta para as imagens geradas")
	pdfPagesCmd.AddCommand(pdfPagesRenderCmd)
	pdfCmd.AddCommand(pdfPagesCmd)
}
