package cli

import (
	"fmt"

	"caramel/internal/output"
	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var pdfImagesOutputDir string

var pdfImagesExtractCmd = &cobra.Command{
	Use:   "extract <entrada.pdf>",
	Short: "Extrai imagens embutidas em um PDF",
	Long: `Extrai os recursos de imagem encontrados em todas as páginas e salva cada um no formato
emitido pelo extrator, sem converter a composição da página.

📚 QUANDO USAR:
Use para recuperar imagens originais embutidas no PDF. Texto e desenhos vetoriais não são convertidos
em imagens; para exportar cada página completa, incluindo texto e vetores, use pdf pages render.
Recursos que o extrator não consegue decodificar são ignorados com aviso. Arquivos existentes nunca
são sobrescritos.

Por padrão, as imagens são salvas em <nome_do_pdf>_embedded_images ao lado do PDF. Use --output-dir
para escolher outra pasta. A operação é local e não usa serviços externos.`,
	Example: `# Extrair imagens embutidas
caramel pdf images extract apostila.pdf

# Escolher a pasta de saída
caramel pdf images extract apostila.pdf --output-dir ./imagens`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		paths, warnings, err := pdf.ExtractEmbeddedImages(args[0], pdfImagesOutputDir)
		if err != nil {
			return err
		}
		status := output.StateSuccess
		summary := fmt.Sprintf("Imagens embutidas extraídas: %d arquivo(s).", len(paths))
		if len(paths) == 0 {
			status = output.StateWarning
			summary = fmt.Sprintf("Nenhuma imagem embutida foi extraída de %s.", args[0])
		}
		if len(warnings) > 0 {
			status = output.StateWarning
		}
		return renderer.Result(output.Result{
			Status:   status,
			Summary:  summary,
			Count:    len(paths),
			Outputs:  paths,
			Warnings: warnings,
		})
	},
}

var pdfImagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Extração de imagens embutidas em PDFs",
	Long:  "Operações com objetos de imagem armazenados dentro de arquivos PDF.",
}

func init() {
	pdfImagesExtractCmd.Flags().StringVarP(&pdfImagesOutputDir, "output-dir", "o", "", "Pasta para as imagens extraídas")
	pdfImagesCmd.AddCommand(pdfImagesExtractCmd)
	pdfCmd.AddCommand(pdfImagesCmd)
}
