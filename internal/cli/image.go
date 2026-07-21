package cli

import (
	"fmt"
	"path/filepath"

	"caramel/internal/config"
	"caramel/internal/tools/ai"

	"github.com/spf13/cobra"
)

var (
	imgOutputDir string
	imgModelName string
	verboseFlag  bool
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Utilitários e ferramentas para processamento de imagens",
	Long:  `Conjunto de comandos para manipular, colorir e otimizar imagens pedagógicas.`,
}

var imageColorizeCmd = &cobra.Command{
	Use:     "colorize <imagem>",
	Aliases: []string{"color", "colorir"},
	Short:   "Colora uma imagem em preto e branco usando IA (OpenRouter / Nano Banana 2)",
	Long:    `Envia uma ilustração ou desenho em preto e branco para a IA e gera uma versão colorida em alta qualidade.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imgPath := args[0]

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
		}

		targetDir := imgOutputDir
		if !cmd.Flags().Changed("output") {
			targetDir = filepath.Dir(imgPath)
		}

		fmt.Printf("🎨 Colorindo imagem '%s' com o modelo '%s'...\n", filepath.Base(imgPath), imgModelName)
		res, err := ai.ColorizeSingleImage(imgPath, targetDir, cfg.OpenRouterAPIKey, imgModelName, verboseFlag)
		if err != nil {
			return err
		}

		fmt.Printf("✅ Sucesso! Imagem colorida salva em: %s\n", res.ColorizedPath)
		return nil
	},
}

func init() {
	imageColorizeCmd.Flags().StringVarP(&imgOutputDir, "output", "o", "", "Diretório de destino (padrão: mesma pasta da imagem original)")
	imageColorizeCmd.Flags().StringVarP(&imgModelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração (padrão: google/gemini-3.1-flash-image)")
	imageColorizeCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Exibe informações detalhadas de depuração e resposta raw da API")

	imageCmd.AddCommand(imageColorizeCmd)
	RootCmd.AddCommand(imageCmd)

	// Atalho direto no Root Cmd para aceitar 'caramel colorize <imagem>'
	RootCmd.AddCommand(imageColorizeCmd)
}
