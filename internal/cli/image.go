package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var (
	imgOutputDir    string
	imgModelName    string
	verboseFlag     bool
	interactiveFlag bool
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Utilitários e ferramentas para processamento de imagens",
	Long:  `Conjunto de comandos para manipular, colorir e otimizar imagens pedagógicas.`,
}

var imageColorizeCmd = &cobra.Command{
	Use:     "colorize <imagem-ou-diretorio>",
	Aliases: []string{"color", "colorir"},
	Short:   "Colora imagem(ns) em preto e branco usando IA (OpenRouter)",
	Long:    `Envia ilustrações em preto e branco para a IA e gera versões coloridas. Aceita um arquivo individual ou um diretório com seleção interativa TUI e preview ANSI no terminal.`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
		}

		// Coleta lista de imagens a processar
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
			// Se múltiplos argumentos forem passados
			for _, arg := range args {
				if isImageFile(arg) {
					candidateImages = append(candidateImages, arg)
				}
			}
			if len(candidateImages) == 0 && isImageFile(inputPath) {
				candidateImages = []string{inputPath}
			}
		}

		// Se houver mais de uma imagem ou a flag --interactive estiver ativa
		var selectedImages []string
		if interactiveFlag || len(candidateImages) > 1 {
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

		// Processa cada imagem selecionada
		fmt.Printf("🎨 Processando %d imagem(ns) com o modelo '%s'...\n", len(selectedImages), imgModelName)

		for i, imgPath := range selectedImages {
			targetDir := imgOutputDir
			if !cmd.Flags().Changed("output") || targetDir == "" {
				targetDir = filepath.Dir(imgPath)
			}

			fmt.Printf("  [%d/%d] Colorindo '%s'...\n", i+1, len(selectedImages), filepath.Base(imgPath))
			res, err := ai.ColorizeSingleImage(imgPath, targetDir, cfg.OpenRouterAPIKey, imgModelName, verboseFlag)
			if err != nil {
				fmt.Printf("❌ Falha ao colorir '%s': %v\n", filepath.Base(imgPath), err)
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
	imageColorizeCmd.Flags().StringVarP(&imgOutputDir, "output", "o", "", "Diretório de destino (padrão: mesma pasta da imagem original)")
	imageColorizeCmd.Flags().StringVarP(&imgModelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração")
	imageColorizeCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Exibe informações detalhadas de depuração e resposta raw da API")
	imageColorizeCmd.Flags().BoolVarP(&interactiveFlag, "interactive", "i", false, "Habilita seleção interativa e preview TUI no terminal")

	imageCmd.AddCommand(imageColorizeCmd)
	RootCmd.AddCommand(imageCmd)

	// Atalho direto no Root Cmd para aceitar 'caramel colorize <imagem>'
	RootCmd.AddCommand(imageColorizeCmd)
}
