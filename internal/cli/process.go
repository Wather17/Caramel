package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
	"caramel/internal/tools/pipeline"

	"github.com/spf13/cobra"
)

var (
	processOutputDir string
	processModelName string
	processMinSize   string
	processVerbose   bool
)

var processCmd = &cobra.Command{
	Use:     "process <arquivo.docx>",
	Aliases: []string{"pipeline", "run"},
	Short:   "Pipeline automatizado: extrai, colore e reconstrói o arquivo .docx com IA",
	Long: `Executa o fluxo completo e automatizado para arquivos .docx:
1. Inspeciona e extrai todas as imagens do documento .docx (ignorando brasões/logos pequenos)
2. Cria uma pasta de saída com nome higienizado (ex: 'imagens <nome_do_arquivo>')
3. Processa e colore cada imagem utilizando IA multimodal (OpenRouter / Nano Banana 2)
4. Ajusta a proporção das imagens para o tamanho original
5. Reconstrói um novo arquivo .docx (ex: '<nome> colorida.docx') com as novas imagens injetadas`,
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

		minSizeBytes, err := docx.ParseSizeInBytes(processMinSize)
		if err != nil {
			return err
		}

		fmt.Printf("🚀 Iniciando Pipeline Automatizado para '%s'...\n", filepath.Base(docxPath))
		fmt.Printf(" ├─ Modelo IA: %s\n", processModelName)
		if minSizeBytes > 0 {
			fmt.Printf(" ├─ Filtro de Tamanho Mínimo: %s (%d bytes)\n", processMinSize, minSizeBytes)
		}

		res, err := pipeline.RunDocxPipeline(docxPath, processOutputDir, cfg.OpenRouterAPIKey, processModelName, minSizeBytes, processVerbose)
		if err != nil {
			return err
		}

		if res.TotalSkipped > 0 {
			fmt.Printf(" ├─ Imagens ignoradas (tamanho < %s): %d\n", processMinSize, res.TotalSkipped)
			for _, img := range res.SkippedImages {
				sizeKB := float64(img.Size) / 1024.0
				fmt.Printf(" │   └─ Ignorada: %s (%.1f KB)\n", img.OriginalName, sizeKB)
			}
		}

		if res.TotalColorized == 0 {
			fmt.Printf("ℹ️  Nenhuma imagem com tamanho >= %s foi encontrada em '%s'.\n", processMinSize, docxPath)
			return nil
		}

		fmt.Printf("✅ Pipeline concluído com sucesso!\n")
		fmt.Printf(" ├─ Total de imagens coloridas/substituídas: %d\n", res.TotalColorized)
		if res.RebuiltDocxPath != "" {
			fmt.Printf(" ├─ Novo arquivo reconstruído: %s\n", res.RebuiltDocxPath)
		}
		fmt.Printf(" └─ Imagens individuais salvas no diretório: %s\n", res.OutputDir)

		return nil
	},
}

func init() {
	processCmd.Flags().StringVarP(&processOutputDir, "output", "o", "", "Diretório onde os arquivos serão salvos (padrão: imagens <nome_do_arquivo>)")
	processCmd.Flags().StringVarP(&processModelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração (padrão: google/gemini-3.1-flash-image)")
	processCmd.Flags().StringVarP(&processMinSize, "min-size", "s", "20KB", "Tamanho mínimo da imagem para ser processada (ex: '20KB', '50KB', '0' para todas)")
	processCmd.Flags().BoolVarP(&processVerbose, "verbose", "v", false, "Exibe informações detalhadas de depuração e resposta raw da API")

	RootCmd.AddCommand(processCmd)
	docxCmd.AddCommand(processCmd)
}
