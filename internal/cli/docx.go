package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"caramel/internal/tools/docx"

	"github.com/spf13/cobra"
)

var (
	outputDir string
	listOnly  bool
)

// docxCmd representa o grupo de comandos relacionados a arquivos .docx
var docxCmd = &cobra.Command{
	Use:   "docx",
	Short: "Ferramentas para manipulação e extração de arquivos .docx",
	Long:  `Conjunto de utilitários para trabalhar com arquivos do Microsoft Word (.docx).`,
}

// docxExtractCmd representa o comando de extração de imagens
var docxExtractCmd = &cobra.Command{
	Use:   "extract <arquivo.docx>",
	Short: "Extrai ou lista imagens contidas em um arquivo .docx",
	Long: `Inspeciona a estrutura interna do arquivo .docx fornecido e extrai todas as imagens 
(diagramas, fotos, gráficos) encontradas na pasta 'word/media/' para um diretório especificado.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		docxPath := args[0]

		// Validação de extensão simples
		if !strings.HasSuffix(strings.ToLower(docxPath), ".docx") {
			return fmt.Errorf("o arquivo '%s' não possui a extensão .docx", docxPath)
		}

		// Se a flag --list (-l) for informada, apenas inspeciona e exibe no terminal
		if listOnly {
			images, err := docx.ListImages(docxPath)
			if err != nil {
				return err
			}

			if len(images) == 0 {
				fmt.Printf("ℹ️  Nenhuma imagem foi encontrada no arquivo '%s'.\n", docxPath)
				return nil
			}

			fmt.Printf("🔍 Imagens encontradas em '%s':\n", filepath.Base(docxPath))
			for i, img := range images {
				sizeKB := float64(img.Size) / 1024.0
				fmt.Printf("  %d. %s (%s, %.1f KB)\n", i+1, img.OriginalName, strings.ToUpper(img.Format), sizeKB)
			}
			fmt.Printf("\nTotal: %d imagem(ns) encontrada(s).\n", len(images))
			return nil
		}

		// Se a flag -o / --output não foi passada explicitamente, gera o nome de pasta dinâmico e higienizado
		targetDir := outputDir
		if !cmd.Flags().Changed("output") {
			targetDir = docx.SanitizeFolderName(docxPath)
		}

		// Processo de extração
		fmt.Printf("🍬 Extraindo imagens de '%s'...\n", filepath.Base(docxPath))
		res, err := docx.ExtractImages(docxPath, targetDir)
		if err != nil {
			return err
		}

		if res.TotalExtracted == 0 {
			fmt.Printf("ℹ️  Nenhuma imagem foi encontrada no arquivo '%s'. Nenhuma imagem extraída.\n", docxPath)
			return nil
		}

		fmt.Printf("✅ Sucesso! %d imagem(ns) extraída(s) para o diretório: %s\n", res.TotalExtracted, targetDir)
		for _, img := range res.Images {
			fmt.Printf("  └─ %s\n", filepath.Join(targetDir, img.OriginalName))
		}

		return nil
	},
}

func init() {
	// Flags do comando extract
	docxExtractCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Diretório onde as imagens extraídas serão salvas (padrão: imagens_<nome_do_arquivo>)")
	docxExtractCmd.Flags().BoolVarP(&listOnly, "list", "l", false, "Apenas lista as imagens encontradas sem extraí-las para o disco")

	// Registra subcomandos
	docxCmd.AddCommand(docxExtractCmd)
	RootCmd.AddCommand(docxCmd)
}
