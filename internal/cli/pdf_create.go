package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/output"
	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var pdfCreateOutput string

var pdfCreateCmd = &cobra.Command{
	Use:   "create <imagem_ou_pasta> [imagens...]",
	Short: "Cria um PDF A4 com uma imagem por página",
	Long: fmt.Sprintf(`Cria um único PDF com uma página A4 por imagem. Cada página acompanha a orientação da imagem,
mantém sua proporção e centraliza o conteúdo sem cortes. Arquivos explícitos seguem a ordem dos
argumentos; imagens encontradas em cada pasta são ordenadas naturalmente.

São aceitos os mesmos formatos raster do comando print 2up (%s).

📚 QUANDO USAR:
Use para montar um documento paginado a partir de atividades exportadas como imagens. O padrão A4
funciona para impressão e o comando processa tudo localmente.`, pdf.SupportedImageExtensionsDescription()),
	Example: `# Criar um PDF com uma imagem por página
caramel pdf create ./atividade.png

# Combinar imagens explícitas na ordem informada
caramel pdf create capa.png atividade.png respostas.png --output apostila.pdf

# Combinar todas as imagens de uma pasta em ordem natural
caramel pdf create ./atividades`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		imagePaths, firstInput, firstStat, err := collectPDFCreateImages(args, renderer)
		if err != nil {
			return err
		}
		outputPath := pdfCreateOutput
		if outputPath == "" {
			outputPath = defaultPDFCreateOutput(args, imagePaths, firstInput, firstStat)
		}

		renderer.Diagnostic("criando PDF com %d imagem(ns)\n", len(imagePaths))
		pageCount, err := pdf.GenerateImagePDF(imagePaths, outputPath, pdf.DefaultOptions())
		if err != nil {
			return fmt.Errorf("falha ao criar PDF a partir das imagens: %w", err)
		}

		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("PDF criado: %d imagem(ns), %d página(s).", len(imagePaths), pageCount),
			Count:   pageCount,
			Outputs: []string{outputPath},
		})
	},
}

func collectPDFCreateImages(args []string, renderer *output.Renderer) ([]string, string, os.FileInfo, error) {
	var imagePaths []string
	firstInput := ""
	var firstStat os.FileInfo

	for argIndex, input := range args {
		realPath, stat, err := pdf.ResolveFuzzyPath(input)
		if err != nil {
			return nil, "", nil, err
		}
		if realPath != input {
			renderer.Diagnostic("caminho ajustado automaticamente para: %s\n", realPath)
		}
		if argIndex == 0 {
			firstInput = realPath
			firstStat = stat
		}

		if !stat.IsDir() {
			if !pdf.IsSupportedImageFile(realPath) {
				return nil, "", nil, fmt.Errorf("o arquivo '%s' precisa ser uma imagem (%s)", realPath, pdf.SupportedImageExtensionsDescription())
			}
			imagePaths = append(imagePaths, realPath)
			continue
		}

		entries, err := os.ReadDir(realPath)
		if err != nil {
			return nil, "", nil, fmt.Errorf("não foi possível ler a pasta '%s': %w", realPath, err)
		}
		folderImages := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !pdf.IsSupportedImageFile(entry.Name()) {
				continue
			}
			folderImages = append(folderImages, filepath.Join(realPath, entry.Name()))
		}
		if len(folderImages) == 0 {
			return nil, "", nil, fmt.Errorf("nenhuma imagem (%s) encontrada na pasta '%s'", pdf.SupportedImageExtensionsDescription(), realPath)
		}
		pdf.SortNatural(folderImages)
		imagePaths = append(imagePaths, folderImages...)
	}

	return imagePaths, firstInput, firstStat, nil
}

func defaultPDFCreateOutput(args, imagePaths []string, firstInput string, firstStat os.FileInfo) string {
	if len(args) == 1 && firstStat != nil {
		if firstStat.IsDir() {
			folderName := filepath.Base(filepath.Clean(firstInput))
			return filepath.Join(filepath.Dir(filepath.Clean(firstInput)), folderName+".pdf")
		}
		return strings.TrimSuffix(firstInput, filepath.Ext(firstInput)) + ".pdf"
	}
	firstImage := imagePaths[0]
	return strings.TrimSuffix(firstImage, filepath.Ext(firstImage)) + "_collection.pdf"
}

func init() {
	pdfCreateCmd.Flags().StringVarP(&pdfCreateOutput, "output", "o", "", "Caminho do PDF final")
	pdfCmd.AddCommand(pdfCreateCmd)
}
