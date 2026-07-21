package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/pipeline"

	"github.com/spf13/cobra"
)

var (
	processOutputDir string
	processModelName string
	processVerbose   bool
)

var processCmd = &cobra.Command{
	Use:     "process <arquivo.docx>",
	Aliases: []string{"pipeline", "run"},
	Short:   "Pipeline automatizado: extrai e colore todas as imagens de um .docx via IA",
	Long: `Executa o fluxo completo e automatizado para arquivos .docx:
1. Inspeciona e extrai todas as imagens do documento .docx
2. Cria uma pasta de saída com nome higienizado (ex: 'imagens <nome_do_arquivo>')
3. Processa e colore cada imagem utilizando IA multimodal (OpenRouter / Nano Banana 2)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		docxPath := args[0]

		if !strings.HasSuffix(strings.ToLower(docxPath), ".docx") {
			return fmt.Errorf("o arquivo '%s' não possui a extensão .docx", docxPath)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
		}

		fmt.Printf("🚀 Iniciando Pipeline Automatizado para '%s'...\n", filepath.Base(docxPath))
		fmt.Printf(" ├─ Modelo IA: %s\n", processModelName)

		res, err := pipeline.RunDocxPipeline(docxPath, processOutputDir, cfg.OpenRouterAPIKey, processModelName, processVerbose)
		if err != nil {
			return err
		}

		if res.TotalColorized == 0 {
			fmt.Printf("ℹ️  Nenhuma imagem foi encontrada ou extraída de '%s'.\n", docxPath)
			return nil
		}

		fmt.Printf("✅ Pipeline concluído com sucesso!\n")
		fmt.Printf(" ├─ Total de imagens processadas: %d\n", res.TotalColorized)
		fmt.Printf(" └─ Imagens salvas no diretório: %s\n", res.OutputDir)

		for i, imgRes := range res.Results {
			fmt.Printf("     %d. %s\n", i+1, imgRes.ColorizedPath)
		}

		return nil
	},
}

func init() {
	processCmd.Flags().StringVarP(&processOutputDir, "output", "o", "", "Diretório onde as imagens coloridas serão salvas (padrão: imagens <nome_do_arquivo>)")
	processCmd.Flags().StringVarP(&processModelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração (padrão: google/gemini-3.1-flash-image)")
	processCmd.Flags().BoolVarP(&processVerbose, "verbose", "v", false, "Exibe informações detalhadas de depuração e resposta raw da API")

	RootCmd.AddCommand(processCmd)
	docxCmd.AddCommand(processCmd)
}
