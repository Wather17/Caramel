package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"caramel/internal/tools/cards"
	"caramel/internal/tools/pdf"

	"github.com/spf13/cobra"
)

var (
	cardsCols      int
	cardsRows      int
	cardsTitle     string
	cardsOutputDir string
	cardsUppercase bool
	cardsHTMLMode  bool
)

var numberPrefixRegex = regexp.MustCompile(`^\d+[_\s-]+`)

// CleanCardName higieniza o nome do arquivo para exibição legível na ficha
func CleanCardName(fileName string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	base = numberPrefixRegex.ReplaceAllString(base, "")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	return strings.Title(strings.TrimSpace(base))
}

var cardsCmd = &cobra.Command{
	Use:     "cards <pasta_ou_imagem>",
	Aliases: []string{"flashcards", "print cards"},
	Short:   "Gera fichas pedagógicas A4 em PDF (padrão) ou HTML, prontas para impressão",
	Long: `Lê imagens de uma pasta (ou imagem avulsa), extrai os nomes e gera um arquivo PDF A4
diagramado em grade com proporção 1:1, legenda do objeto abaixo da imagem (caixa alta por padrão).
Use --html para gerar o layout HTML/Tailwind (abrível no navegador para edição).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputArg := args[0]

		// Validação da grade de fichas (1 a 6 colunas/linhas)
		if err := validateCardGrid(cardsCols, cardsRows); err != nil {
			return err
		}

		realPath, stat, err := pdf.ResolveFuzzyPath(inputArg)
		if err != nil {
			return err
		}
		inputPath := realPath

		var cardItems []cards.CardItem
		var defaultOutPath string

		if stat.IsDir() {
			entries, err := os.ReadDir(inputPath)
			if err != nil {
				return fmt.Errorf("erro ao ler diretório '%s': %w", inputPath, err)
			}

			var fileNames []string
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(e.Name()))
				if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
					fileNames = append(fileNames, e.Name())
				}
			}

			if len(fileNames) == 0 {
				return fmt.Errorf("nenhuma imagem (PNG, JPG, WEBP) encontrada em '%s'", inputPath)
			}

			pdf.SortNatural(fileNames)
			for _, fn := range fileNames {
				imgPath := filepath.Join(inputPath, fn)
				cardItems = append(cardItems, cards.CardItem{
					Name:      CleanCardName(fn),
					ImagePath: imgPath,
				})
			}

			folderName := filepath.Base(filepath.Clean(inputPath))
			defaultOutPath = filepath.Join(inputPath, fmt.Sprintf("%s_fichas_a4.%s", folderName, outputExt()))
			if cardsTitle == "" {
				cardsTitle = strings.Title(strings.ReplaceAll(folderName, "_", " "))
			}
		} else {
			ext := strings.ToLower(filepath.Ext(inputPath))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
				return fmt.Errorf("o arquivo fornecido precisa ser uma imagem (PNG, JPG, WEBP)")
			}

			cardItems = append(cardItems, cards.CardItem{
				Name:      CleanCardName(filepath.Base(inputPath)),
				ImagePath: inputPath,
			})
			dir := filepath.Dir(inputPath)
			baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
			defaultOutPath = filepath.Join(dir, fmt.Sprintf("%s_fichas_a4.%s", baseName, outputExt()))
		}

		outPath := defaultOutPath
		if cardsOutputDir != "" {
			if outStat, err := os.Stat(cardsOutputDir); err == nil && outStat.IsDir() {
				outPath = filepath.Join(cardsOutputDir, filepath.Base(defaultOutPath))
			} else {
				outPath = cardsOutputDir
			}
		}

		opts := cards.SheetOptions{
			Columns:   cardsCols,
			Rows:      cardsRows,
			Title:     cardsTitle,
			Uppercase: cardsUppercase,
		}

		fmt.Printf("🖨️ Gerando layout A4 de fichas para %d imagem(ns)...\n", len(cardItems))
		fmt.Printf(" ├─ Grade: %d colunas x %d linhas (%d fichas por folha)\n", opts.Columns, opts.Rows, opts.Columns*opts.Rows)

		if cardsHTMLMode {
			if err := cards.GenerateCardsHTML(cardItems, outPath, opts); err != nil {
				return fmt.Errorf("falha ao gerar fichas HTML: %w", err)
			}
			fmt.Printf("✅ Sucesso! Fichas A4 geradas em:\n   👉 %s\n", outPath)
			fmt.Println("💡 Dica: Abra o arquivo no navegador e pressione Ctrl+P para imprimir ou salvar como PDF.")
			return nil
		}

		if err := cards.GenerateCardsPDF(cardItems, outPath, opts); err != nil {
			return fmt.Errorf("falha ao gerar fichas PDF: %w", err)
		}

		fmt.Printf("✅ Sucesso! Fichas A4 geradas em:\n   👉 %s\n", outPath)
		fmt.Println("💡 Dica: O PDF já está pronto para impressão direta (Ctrl+P no leitor ou envie para a impressora).")
		return nil
	},
}

// outputExt retorna a extensão do arquivo de saída conforme o modo escolhido
func outputExt() string {
	if cardsHTMLMode {
		return "html"
	}
	return "pdf"
}

// validateCardGrid valida a faixa de colunas/linhas da grade de fichas
func validateCardGrid(cols, rows int) error {
	if cols < 1 || cols > 6 {
		return fmt.Errorf("número de colunas inválido (%d): use um valor entre 1 e 6", cols)
	}
	if rows < 1 || rows > 6 {
		return fmt.Errorf("número de linhas inválido (%d): use um valor entre 1 e 6", rows)
	}
	return nil
}

func init() {
	cardsCmd.Flags().IntVarP(&cardsCols, "cols", "c", 2, "Número de colunas na folha A4 (1 a 6)")
	cardsCmd.Flags().IntVarP(&cardsRows, "rows", "r", 3, "Número de linhas na folha A4 (1 a 6)")
	cardsCmd.Flags().StringVarP(&cardsTitle, "title", "t", "", "Título exibido no cabeçalho de cada folha")
	cardsCmd.Flags().StringVarP(&cardsOutputDir, "output", "o", "", "Caminho do arquivo de saída ou diretório de destino")
	cardsCmd.Flags().BoolVarP(&cardsUppercase, "uppercase", "u", true, "Exibe nomes das fichas em caixa alta (ex: MAÇÃ)")
	cardsCmd.Flags().BoolVar(&cardsHTMLMode, "html", false, "Gera o layout HTML/Tailwind (para abrir no navegador) em vez do PDF")

	RootCmd.AddCommand(cardsCmd)
}
