package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/pdf"
	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var (
	imgOutputDir    string
	imgModelName    string
	imgMinSize      string
	imgTriageModel  string
	imgNoTriage     bool
	verboseFlag     bool
	interactiveFlag bool
	allFlag         bool
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Utilitários e ferramentas para processamento de imagens",
	Long:  `Conjunto de comandos para manipular, colorir e otimizar imagens pedagógicas.`,
}

var imageColorizeCmd = &cobra.Command{
	Use:     "colorize <imagem|diretorio|arquivo.docx>",
	Aliases: []string{"color", "colorir"},
	Short:   "Colora imagem(ns) ou documentos .docx em preto e branco usando IA (OpenRouter)",
	Long: `Envia ilustrações em preto e branco para a IA e gera versões coloridas.
Aceita arquivos de imagem individuais (PNG, JPG, WEBP), pastas inteiras ou arquivos .docx.

Ao receber um arquivo .docx:
- Executa o pipeline automatizado: extrai as imagens, colore via IA, ajusta proporções e gera um novo arquivo .docx reconstruído ('<nome> colorida.docx') além de salvar as imagens coloridas.
- Suporta a flag '-i' / '--interactive' para seleção interativa com preview ANSI no terminal.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]

		// Se o alvo for um arquivo .docx, executa o pipeline unificado de DOCX
		if strings.ToLower(filepath.Ext(inputPath)) == ".docx" {
			effectiveMinSize := imgMinSize
			if allFlag && !cmd.Flags().Changed("min-size") {
				effectiveMinSize = "0"
			}

			isInteractive := interactiveFlag
			if allFlag {
				isInteractive = false
			}

			return RunProcessDocx(ProcessDocxOptions{
				DocxPath:    inputPath,
				OutputDir:   imgOutputDir,
				ModelName:   imgModelName,
				MinSize:     effectiveMinSize,
				Interactive: isInteractive,
				Verbose:     verboseFlag,
				TriageModel: imgTriageModel,
				NoTriage:    imgNoTriage,
			})
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
		}

		var candidateImages []string

		info, err := os.Stat(inputPath)
		if err != nil {
			return fmt.Errorf("falha ao acessar '%s': %w", inputPath, err)
		}

		if info.IsDir() {
			entries, err := os.ReadDir(inputPath)
			if err != nil {
				return fmt.Errorf("falha ao ler diretório '%s': %w", inputPath, err)
			}

			for _, entry := range entries {
				if !entry.IsDir() && isImageFile(entry.Name()) {
					candidateImages = append(candidateImages, filepath.Join(inputPath, entry.Name()))
				}
			}

			if len(candidateImages) == 0 {
				return fmt.Errorf("nenhuma imagem (PNG, JPG, WEBP) encontrada no diretório '%s'", inputPath)
			}
		} else {
			for _, arg := range args {
				if isImageFile(arg) {
					candidateImages = append(candidateImages, arg)
				}
			}
			if len(candidateImages) == 0 && isImageFile(inputPath) {
				candidateImages = []string{inputPath}
			}
			if len(candidateImages) == 0 {
				return fmt.Errorf("o arquivo '%s' não é uma imagem válida (PNG, JPG, WEBP) ou documento .docx", inputPath)
			}
		}

		// Ordena a lista de imagens de forma numérica natural (ex: image1, image2, ..., image10)
		pdf.SortNatural(candidateImages)

		// Se a flag --all (-a) for passada, seleciona todas sem abrir formulário interativo
		var selectedImages []string
		if allFlag {
			selectedImages = candidateImages
		} else if interactiveFlag || len(candidateImages) > 1 {
			selected, err := ui.SelectImageFilesWithPreviewInteractive(candidateImages)
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				fmt.Println("Nenhuma imagem selecionada para coloração.")
				return nil
			}
			selectedImages = selected
		} else {
			selectedImages = candidateImages
		}

		// Determina diretório final de saída para imagens coloridas
		defaultOutputDir := filepath.Dir(inputPath)

		fmt.Printf("🎨 Processando %d imagem(ns) com o modelo '%s'...\n", len(selectedImages), imgModelName)

		for i, imgPath := range selectedImages {
			targetDir := imgOutputDir
			if !cmd.Flags().Changed("output") || targetDir == "" {
				targetDir = defaultOutputDir
			}

			fmt.Printf("  [%d/%d] Colorindo '%s'...\n", i+1, len(selectedImages), filepath.Base(imgPath))
			res, err := ai.ColorizeSingleImage(imgPath, ai.ColorizeOptions{
				OutputDir:     targetDir,
				APIKey:        cfg.OpenRouterAPIKey,
				Model:         imgModelName,
				TriageModel:   imgTriageModel,
				DisableTriage: imgNoTriage,
				Verbose:       verboseFlag,
			})
			if err != nil {
				fmt.Printf("❌ Falha ao colorir '%s': %v\n", filepath.Base(imgPath), err)
				continue
			}

			if res.Skipped {
				fmt.Printf("  ⏭️  Pulada pela triagem: %s\n", res.SkipReason)
				continue
			}

			fmt.Printf("  ✅ Salvo em: %s\n", res.ColorizedPath)
		}

		return nil
	},
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp"
}

func init() {
	imageColorizeCmd.Flags().StringVarP(&imgOutputDir, "output", "o", "", "Diretório de destino (padrão: pasta da imagem original ou pasta do docx)")
	imageColorizeCmd.Flags().StringVarP(&imgModelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração")
	imageColorizeCmd.Flags().StringVarP(&imgMinSize, "min-size", "s", "20KB", "Tamanho mínimo da imagem ao processar .docx (ex: '20KB', '50KB', '0' para todas)")
	imageColorizeCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Exibe informações detalhadas de depuração e resposta raw da API")
	imageColorizeCmd.Flags().BoolVarP(&interactiveFlag, "interactive", "i", false, "Habilita seleção interativa e preview TUI no terminal")
	imageColorizeCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Colora todas as imagens encontradas sem abrir formulário de seleção")
	imageColorizeCmd.Flags().StringVar(&imgTriageModel, "triage-model", ai.DefaultTriageModel, "Modelo de IA de visão usado na triagem de economia antes da coloração")
	imageColorizeCmd.Flags().BoolVar(&imgNoTriage, "no-triage", false, "Desativa a triagem e colora todas as imagens selecionadas diretamente")

	imageCmd.AddCommand(imageColorizeCmd)
	RootCmd.AddCommand(imageCmd)

	// Atalho direto no Root Cmd para aceitar 'caramel colorize <imagem|docx>'
	RootCmd.AddCommand(imageColorizeCmd)
}
