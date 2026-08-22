package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
	"caramel/internal/tools/pipeline"
	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var (
	outputDir       string
	listOnly        bool
	colorize        bool
	modelName       string
	minSizeStr      string
	docxVerbose     bool
	docxInteractive bool
	docxTriageModel string
	docxNoTriage    bool
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
	Short: "Extrai, lista ou colore imagens contidas em um arquivo .docx",
	Long: `Inspeciona a estrutura interna do arquivo .docx fornecido e extrai todas as imagens 
(diagramas, fotos, gráficos) encontradas na pasta 'word/media/' para um diretório especificado.
Com a flag --colorize (-c), as imagens em preto e branco são coloridas automaticamente via IA (OpenRouter).
Com a flag --interactive (-i), exibe um menu interativo para escolher quais imagens extrair/colorir.`,
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

		minSizeBytes, err := docx.ParseSizeInBytes(minSizeStr)
		if err != nil {
			return err
		}

		// Se a flag -o / --output não foi passada explicitamente, gera o nome de pasta dinâmico e higienizado
		targetDir := outputDir
		if !cmd.Flags().Changed("output") {
			targetDir = docx.SanitizeFolderName(docxPath)
		}

		// Se a flag --interactive (-i) estiver ativada, exibe a TUI de seleção de imagens
		if docxInteractive {
			allImages, err := docx.ListImages(docxPath)
			if err != nil {
				return err
			}

			if len(allImages) == 0 {
				fmt.Printf("ℹ️  Nenhuma imagem foi encontrada no arquivo '%s'.\n", docxPath)
				return nil
			}

			keptImages, skippedImages := docx.FilterImagesByMinSize(allImages, minSizeBytes)
			if len(keptImages) == 0 {
				fmt.Printf("ℹ️  Nenhuma imagem com tamanho >= %s foi encontrada em '%s'.\n", minSizeStr, docxPath)
				return nil
			}

			selectedImages, err := ui.SelectImagesInteractive(keptImages)
			if err != nil {
				return err
			}

			if len(selectedImages) == 0 {
				fmt.Println("ℹ️  Nenhuma imagem foi selecionada.")
				return nil
			}

			if colorize {
				cfg, err := config.LoadConfig()
				if err != nil {
					return err
				}
				if cfg.OpenRouterAPIKey == "" {
					return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
				}

			fmt.Printf("🎨 Processando %d imagem(ns) selecionada(s) com IA...\n", len(selectedImages))
			pipeRes, err := pipeline.RunDocxPipelineSelected(docxPath, targetDir, cfg.OpenRouterAPIKey, modelName, selectedImages, docxVerbose, docxTriageModel, docxNoTriage)
			if err != nil {
				return err
			}

			printTriageSummary(pipeRes)

				fmt.Printf("✅ Sucesso! %d imagem(ns) colorida(s) salvas em: %s\n", pipeRes.TotalColorized, pipeRes.OutputDir)
				for _, res := range pipeRes.Results {
					fmt.Printf("  └─ %s\n", res.ColorizedPath)
				}
				return nil
			}

			fmt.Printf("🍬 Extraindo %d imagem(ns) selecionada(s)...\n", len(selectedImages))
			res, err := docx.ExtractImagesFromList(docxPath, targetDir, selectedImages)
			if err != nil {
				return err
			}

			if len(skippedImages) > 0 {
				fmt.Printf(" ├─ Imagens ignoradas pelo filtro de tamanho (< %s): %d\n", minSizeStr, len(skippedImages))
			}

			fmt.Printf("✅ Sucesso! %d imagem(ns) extraída(s) para o diretório: %s\n", res.TotalExtracted, targetDir)
			for _, img := range res.Images {
				fmt.Printf("  └─ %s\n", filepath.Join(targetDir, img.OriginalName))
			}
			return nil
		}

		// Se a flag --colorize (-c) foi ativada
		if colorize {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			if cfg.OpenRouterAPIKey == "" {
				return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>' para poder utilizar a IA de coloração")
			}

		fmt.Printf("🎨 Extraindo e colorindo imagens de '%s' usando o modelo '%s'...\n", filepath.Base(docxPath), modelName)
		pipeRes, err := pipeline.RunDocxPipeline(docxPath, targetDir, cfg.OpenRouterAPIKey, modelName, minSizeBytes, docxVerbose, docxTriageModel, docxNoTriage)
		if err != nil {
			return err
		}

		if pipeRes.TotalSkipped > 0 {
			fmt.Printf(" ├─ Imagens ignoradas (tamanho < %s): %d\n", minSizeStr, pipeRes.TotalSkipped)
		}

		printTriageSummary(pipeRes)

if pipeRes.TotalColorized == 0 {
			if pipeRes.TotalTriageSkipped > 0 {
				fmt.Printf("ℹ️  Nenhuma imagem foi aprovada pela triagem do arquivo '%s' (todas foram puladas).\n", docxPath)
			} else {
				fmt.Printf("ℹ️  Nenhuma imagem com tamanho >= %s foi extraída/colorida do arquivo '%s'.\n", minSizeStr, docxPath)
			}
			return nil
		}

			fmt.Printf("✅ Sucesso! %d imagem(ns) colorida(s) salvas em: %s\n", pipeRes.TotalColorized, pipeRes.OutputDir)
			for _, res := range pipeRes.Results {
				fmt.Printf("  └─ %s\n", res.ColorizedPath)
			}
			return nil
		}

		// Processo de extração padrão (sem coloração)
		fmt.Printf("🍬 Extraindo imagens de '%s'...\n", filepath.Base(docxPath))
		res, err := docx.ExtractImagesFiltered(docxPath, targetDir, minSizeBytes)
		if err != nil {
			return err
		}

		if res.TotalSkipped > 0 {
			fmt.Printf(" ├─ Imagens ignoradas (tamanho < %s): %d\n", minSizeStr, res.TotalSkipped)
			for _, img := range res.SkippedImages {
				sizeKB := float64(img.Size) / 1024.0
				fmt.Printf(" │   └─ Ignorada: %s (%.1f KB)\n", img.OriginalName, sizeKB)
			}
		}

		if res.TotalExtracted == 0 {
			fmt.Printf("ℹ️  Nenhuma imagem com tamanho >= %s foi encontrada no arquivo '%s'. Nenhuma imagem extraída.\n", minSizeStr, docxPath)
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
	docxExtractCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Diretório onde as imagens serão salvas (padrão: imagens <nome_do_arquivo>)")
	docxExtractCmd.Flags().BoolVarP(&listOnly, "list", "l", false, "Apenas lista as imagens encontradas sem extraí-las para o disco")
	docxExtractCmd.Flags().BoolVarP(&colorize, "colorize", "c", false, "Colora automaticamente as imagens extraídas via IA (OpenRouter)")
	docxExtractCmd.Flags().StringVarP(&modelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração (padrão: google/gemini-3.1-flash-image-preview)")
	docxExtractCmd.Flags().StringVarP(&minSizeStr, "min-size", "s", "0", "Tamanho mínimo da imagem para ser extraída (ex: '20KB', '50KB', '0' para todas)")
	docxExtractCmd.Flags().BoolVarP(&docxVerbose, "verbose", "v", false, "Exibe informações detalhadas de depuração e resposta raw da API")
	docxExtractCmd.Flags().BoolVarP(&docxInteractive, "interactive", "i", false, "Exibe menu interativo para selecionar quais imagens extrair/processar")
	docxExtractCmd.Flags().StringVar(&docxTriageModel, "triage-model", ai.DefaultTriageModel, "Modelo de IA de visão usado na triagem de economia antes da coloração")
	docxExtractCmd.Flags().BoolVar(&docxNoTriage, "no-triage", false, "Desativa a triagem e colora todas as imagens elegíveis diretamente")

	// Registra subcomandos
	docxCmd.AddCommand(docxExtractCmd)
	RootCmd.AddCommand(docxCmd)
}
