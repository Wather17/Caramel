package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var (
	pdfOutputDir       string
	pdfDrawCutLine     bool
	pdfMarginMM        float64
	pdfDuplicateSingle bool
)

var pdfCmd = &cobra.Command{
	Use:   "pdf",
	Short: "Comandos para manipulação e geração de PDFs para impressão",
	Long:  `Subcomandos para montagem de PDFs pedagógicos em layout 2-up (2 por folha), impressão e otimização.`,
}

var pdf2UpCmd = &cobra.Command{
	Use:     "2up <imagem_ou_pasta>",
	Aliases: []string{"print", "layout"},
	Short:   "Gera um PDF A4 Paisagem com 2 atividades lado a lado na mesma folha (economiza papel)",
	Long: `Lê uma imagem isolada ou uma pasta de imagens (PNG, JPG, WEBP) e monta um PDF em orientação Paisagem 
com 2 páginas/atividades por folha, incluindo margens ajustáveis e linha de corte central orientativa.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]

		stat, err := os.Stat(inputPath)
		if err != nil {
			return fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
		}

		var imagePaths []string
		var defaultPdfName string

		if stat.IsDir() {
			// Lê todas as imagens Válidas da pasta
			entries, err := os.ReadDir(inputPath)
			if err != nil {
				return fmt.Errorf("erro ao ler pasta '%s': %w", inputPath, err)
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
					imagePaths = append(imagePaths, filepath.Join(inputPath, entry.Name()))
				}
			}

			if len(imagePaths) == 0 {
				return fmt.Errorf("nenhuma imagem (PNG, JPG, WEBP) encontrada na pasta '%s'", inputPath)
			}

			sort.Strings(imagePaths)
			folderName := filepath.Base(filepath.Clean(inputPath))
			defaultPdfName = filepath.Join(inputPath, fmt.Sprintf("%s_2up.pdf", folderName))
		} else {
			// Arquivo único
			ext := strings.ToLower(filepath.Ext(inputPath))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
				return fmt.Errorf("o arquivo fornecido precisa ser uma imagem (PNG, JPG, WEBP)")
			}

			imagePaths = []string{inputPath}
			baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
			dir := filepath.Dir(inputPath)
			defaultPdfName = filepath.Join(dir, fmt.Sprintf("%s_2up.pdf", baseName))
		}

		// Define o caminho final do PDF de saída
		outputPath := defaultPdfName
		if pdfOutputDir != "" {
			// Se o usuário passou um diretório existente como output
			if outStat, err := os.Stat(pdfOutputDir); err == nil && outStat.IsDir() {
				outputPath = filepath.Join(pdfOutputDir, filepath.Base(defaultPdfName))
			} else {
				outputPath = pdfOutputDir
			}
		}

		opts := pdf.Options{
			DrawCutLine:     pdfDrawCutLine,
			MarginMM:        pdfMarginMM,
			DuplicateSingle: pdfDuplicateSingle,
		}

		fmt.Printf("🚀 Gerando PDF 2-up a partir de %d imagem(ns)...\n", len(imagePaths))
		for _, img := range imagePaths {
			fmt.Printf(" ├─ %s\n", filepath.Base(img))
		}

		if err := pdf.Generate2UpPDF(imagePaths, outputPath, opts); err != nil {
			return fmt.Errorf("falha ao gerar PDF 2-up: %w", err)
		}

		fmt.Printf("✅ Sucesso! PDF de impressão gerado e salvo em:\n   👉 %s\n", outputPath)
		return nil
	},
}

func init() {
	pdf2UpCmd.Flags().StringVarP(&pdfOutputDir, "output", "o", "", "Caminho do arquivo PDF ou pasta de destino")
	pdf2UpCmd.Flags().BoolVarP(&pdfDrawCutLine, "cut-line", "l", true, "Desenha a linha tracejada central de corte")
	pdf2UpCmd.Flags().Float64VarP(&pdfMarginMM, "margin", "m", 5.0, "Margem externa da página em milímetros (mm)")
	pdf2UpCmd.Flags().BoolVarP(&pdfDuplicateSingle, "duplicate", "d", true, "Duplica imagens ímpares ou isoladas no segundo slot da folha")

	pdfCmd.AddCommand(pdf2UpCmd)
	RootCmd.AddCommand(pdfCmd)

	// Atalhos no RootCmd para acesso direto
	RootCmd.AddCommand(pdf2UpCmd)
}
