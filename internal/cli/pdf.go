package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var (
	pdfOutputDir       string
	pdfDrawCutLine     bool
	pdfMarginMM        float64
	pdfDuplicateSingle bool
	pdfAutoRotate      bool
	pdfRotateThreshold float64
	pdfFitMode         string
	pdfOptimize        bool
	pdfMaxDPI          int
	pdfQuality         int
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
com 2 páginas/atividades por folha, incluindo margens ajustáveis e linha de corte central orientativa.

📚 QUANDO USAR:
Use para preparar avaliações, atividades ou apostilas em formato 2 por folha, imprimindo duas
atividades lado a lado em uma única folha A4 — economiza papel e tinta. Imagens horizontais
(landscape) são rotacionadas automaticamente para aproveitar a área útil, e imagens pesadas do
Figma são comprimidas em memória para gerar PDFs leves (~500KB).`,
	Example: `# Gerar PDF 2-up de uma pasta de atividades exportadas do Figma
caramel 2up ./atividades_figma

# Gerar 2-up de uma prova única, duplicando-a no segundo slot
caramel 2up prova_historia.png

# Forçar preenchimento total do slot (cover) sem margem em branco
caramel 2up ./fichas_estudo -f cover`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputArg := args[0]

		realPath, stat, err := pdf.ResolveFuzzyPath(inputArg)
		if err != nil {
			return err
		}
		if realPath != inputArg {
			fmt.Printf("ℹ️ Caminho ajustado automaticamente para: '%s'\n", realPath)
		}
		inputPath := realPath

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

			pdf.SortNatural(imagePaths)
			folderName := pdf.Clean2UpSuffix(filepath.Base(filepath.Clean(inputPath)))
			defaultPdfName = filepath.Join(inputPath, fmt.Sprintf("%s_2up.pdf", folderName))
		} else {
			// Arquivo único
			ext := strings.ToLower(filepath.Ext(inputPath))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
				return fmt.Errorf("o arquivo fornecido precisa ser uma imagem (PNG, JPG, WEBP)")
			}

			imagePaths = []string{inputPath}
			baseName := pdf.Clean2UpSuffix(strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath)))
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
			AutoRotate:      pdfAutoRotate,
			RotateThreshold: pdfRotateThreshold,
			FitMode:         pdfFitMode,
			Optimize:        pdfOptimize,
			MaxDPI:          pdfMaxDPI,
			Quality:         pdfQuality,
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
	pdf2UpCmd.Flags().BoolVarP(&pdfAutoRotate, "auto-rotate", "r", true, "Rotaciona imagens landscape automaticamente para maximizar área útil")
	pdf2UpCmd.Flags().Float64VarP(&pdfRotateThreshold, "rotate-threshold", "t", 15.0, "Porcentagem mínima de ganho de área útil para autorizar a rotação")
	pdf2UpCmd.Flags().StringVarP(&pdfFitMode, "fit", "f", "contain", "Modo de encaixe da imagem no slot: contain (sem cortes) ou cover (preenchimento total)")
	pdf2UpCmd.Flags().BoolVarP(&pdfOptimize, "optimize", "O", true, "Redimensiona (300 DPI) e comprime imagens pesadas em memória para reduzir o tamanho do PDF")
	pdf2UpCmd.Flags().IntVar(&pdfMaxDPI, "max-dpi", 300, "Resolução máxima em DPI para renderização de imagens no PDF")
	pdf2UpCmd.Flags().IntVarP(&pdfQuality, "quality", "q", 85, "Qualidade de compressão JPEG de 1 a 100 (padrão: 85)")

	pdfCmd.AddCommand(pdf2UpCmd)
	RootCmd.AddCommand(pdfCmd)

	// Atalhos no RootCmd para acesso direto
	RootCmd.AddCommand(pdf2UpCmd)
}
