package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/cards"
	"caramel/internal/tools/pdf"
	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var (
	genItemsStr    string
	genFilePath    string
	genTheme       string
	genCount       int
	genStyle       string
	genCustomStyle string
	genOutputDir   string
	genWorkers     int
	genPreview     bool
	genCards       bool
	gen2UpPDF      bool
	genVerbose     bool
	genModelName   string
	genTextModel   string
)

var imageGenerateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen"},
	Short:   "Gera ilustrações e objetos pedagógicos em lote com prompts sintetizados por IA",
	Long: `Harness de geração em lote de imagens pedagógicas.
Permite informar uma lista de palavras (ex: frutas, legumes, animais, ações), um arquivo de texto ou um tema descritivo.
A IA sintetiza e padroniza os prompts automaticamente e um motor em Go com concorrência adaptativa gera as imagens rapidamente.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var rawItems []string

		// 1. Prioridade para argumentos posicionais diretos
		if len(args) > 0 {
			for _, arg := range args {
				parts := strings.Split(arg, ",")
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if trimmed != "" {
						rawItems = append(rawItems, trimmed)
					}
				}
			}
		}

		// 2. Se a flag --items for informada
		if genItemsStr != "" {
			parts := strings.Split(genItemsStr, ",")
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					rawItems = append(rawItems, trimmed)
				}
			}
		}

		// 3. Se a flag --file for informada
		if genFilePath != "" {
			f, err := os.Open(genFilePath)
			if err != nil {
				return fmt.Errorf("falha ao abrir arquivo de itens '%s': %w", genFilePath, err)
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					parts := strings.Split(line, ",")
					for _, p := range parts {
						trimmed := strings.TrimSpace(p)
						if trimmed != "" {
							rawItems = append(rawItems, trimmed)
						}
					}
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("erro ao ler arquivo '%s': %w", genFilePath, err)
			}
		}

		// Validação de entrada
		if len(rawItems) == 0 && genTheme == "" {
			return fmt.Errorf("nenhum item ou tema informado. Use --items, --file, --theme ou passe as palavras como argumentos (ex: caramel generate maçã, banana)")
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
		}

		client, err := ai.NewClient(cfg.OpenRouterAPIKey)
		if err != nil {
			return err
		}
		client.Verbose = genVerbose

		harnessCfg := ai.HarnessConfig{
			Items:       rawItems,
			Theme:       genTheme,
			Count:       genCount,
			Style:       genStyle,
			CustomStyle: genCustomStyle,
			OutputDir:   genOutputDir,
			MaxWorkers:  genWorkers,
			TextModel:   genTextModel,
			ImageModel:  genModelName,
			Verbose:     genVerbose,
		}

		// Estágio 1: Síntese de prompts
		if genTheme != "" && len(rawItems) == 0 {
			fmt.Printf("🧠 Sintetizando %d itens e prompts para o tema '%s' (estilo: %s)...\n", genCount, genTheme, genStyle)
		} else {
			fmt.Printf("🧠 Sintetizando prompts padronizados para %d item(ns) (estilo: %s)...\n", len(rawItems), genStyle)
		}

		items, err := ai.SynthesizePrompts(harnessCfg, client)
		if err != nil {
			return err
		}

		fmt.Printf("📋 Lista sintetizada com sucesso (%d itens):\n", len(items))
		for _, it := range items {
			fmt.Printf("  %02d. %s (%s)\n", it.Index, it.Name, it.Slug)
		}

		// Define pasta final de destino
		targetDir := genOutputDir
		if targetDir == "" {
			themeSlug := "itens"
			if genTheme != "" {
				themeSlug = strings.ToLower(strings.ReplaceAll(genTheme, " ", "_"))
			} else if len(items) > 0 {
				themeSlug = items[0].Slug
			}
			targetDir = fmt.Sprintf("./imagens_%s", themeSlug)
			harnessCfg.OutputDir = targetDir
		}

		// Estágio 2: Execução com concorrência adaptativa
		workers, delay := ai.CalculateConcurrencyDecision(len(items), genWorkers)
		fmt.Printf("\n🚀 Iniciando motor de geração (Workers: %d, Delay: %v)...\n", workers, delay)
		fmt.Printf("📁 Pasta de saída: %s\n\n", targetDir)

		progressFunc := func(ev ai.HarnessProgressEvent) {
			if ev.CurrentStep == "saved" {
				fmt.Printf("[%d/%d] ✅ Gerado: %s -> %s\n", ev.Completed, ev.Total, ev.Item.Name, filepath.Base(ev.Item.ImagePath))
				if genPreview && ev.Item.ImagePath != "" {
					ansiArt, err := ui.RenderImageFileToANSI(ev.Item.ImagePath, 40, 20)
					if err == nil && ansiArt != "" {
						fmt.Println(ansiArt)
					}
				}
			} else if ev.CurrentStep == "error" {
				fmt.Printf("[%d/%d] ❌ Erro ao gerar %s: %s\n", ev.Completed, ev.Total, ev.Item.Name, ev.Item.Error)
			}
		}

		results, err := ai.ExecuteGenerationHarness(items, harnessCfg, client, progressFunc)
		if err != nil {
			return err
		}

		var successfulPaths []string
		successCount := 0
		failCount := 0

		for _, res := range results {
			if res.Status == "done" && res.ImagePath != "" {
				successCount++
				successfulPaths = append(successfulPaths, res.ImagePath)
			} else {
				failCount++
			}
		}

		fmt.Printf("\n🏁 Geração concluída!\n")
		fmt.Printf(" ├─ Total de imagens geradas com sucesso: %d/%d\n", successCount, len(results))
		if failCount > 0 {
			fmt.Printf(" ├─ Falhas: %d\n", failCount)
		}
		fmt.Printf(" └─ Diretório: %s\n", targetDir)

		// Estágio 3: Geração automática de Fichas Pedagógicas A4 (HTML/Tailwind)
		if genCards && len(results) > 0 {
			var cardItems []cards.CardItem
			for _, res := range results {
				if res.Status == "done" && res.ImagePath != "" {
					cardItems = append(cardItems, cards.CardItem{
						Name:      res.Name,
						ImagePath: res.ImagePath,
					})
				}
			}

			if len(cardItems) > 0 {
				htmlOutPath := filepath.Join(targetDir, "fichas_a4.html")
				title := "Coleção Pedagógica"
				if genTheme != "" {
					title = strings.Title(genTheme)
				}
				cardOpts := cards.DefaultOptions()
				cardOpts.Title = title

				if err := cards.GenerateCardsHTML(cardItems, htmlOutPath, cardOpts); err != nil {
					fmt.Printf("⚠️ Aviso: Falha ao gerar fichas A4: %v\n", err)
				} else {
					fmt.Printf("🖨️ Layout de Fichas A4 gerado para impressão:\n   👉 %s\n", htmlOutPath)
				}
			}
		}

		// Estágio 4 Opcional: Montagem em PDF 2-up
		if gen2UpPDF && len(successfulPaths) > 0 {
			pdf.SortNatural(successfulPaths)
			pdfOutPath := filepath.Join(targetDir, "atividades_2up.pdf")

			pdfOpts := pdf.Options{
				DrawCutLine:     true,
				MarginMM:        5.0,
				DuplicateSingle: true,
				AutoRotate:      true,
				RotateThreshold: 15.0,
				FitMode:         "contain",
				Optimize:        true,
				MaxDPI:          300,
				Quality:         85,
			}

			fmt.Printf("\n📄 Compilando %d imagens em PDF 2-up para impressão...\n", len(successfulPaths))
			if err := pdf.Generate2UpPDF(successfulPaths, pdfOutPath, pdfOpts); err != nil {
				fmt.Printf("⚠️ Aviso: Falha ao compilar PDF 2-up: %v\n", err)
			} else {
				fmt.Printf("✅ PDF 2-up gerado com sucesso:\n   👉 %s\n", pdfOutPath)
			}
		}

		return nil
	},
}

func init() {
	imageGenerateCmd.Flags().StringVarP(&genItemsStr, "items", "i", "", "Lista de itens ou palavras separadas por vírgula (ex: 'maçã, banana, uva')")
	imageGenerateCmd.Flags().StringVarP(&genFilePath, "file", "f", "", "Caminho para arquivo .txt com itens linha por linha")
	imageGenerateCmd.Flags().StringVarP(&genTheme, "theme", "t", "", "Tema descritivo para a IA escolher e gerar os itens automaticamente")
	imageGenerateCmd.Flags().IntVarP(&genCount, "count", "n", 10, "Quantidade de itens a gerar quando utilizado com --theme")
	imageGenerateCmd.Flags().StringVarP(&genStyle, "style", "s", "clipart", "Estilo visual: clipart, vector, 3d-cute, coloring, realistic")
	imageGenerateCmd.Flags().StringVar(&genCustomStyle, "custom-style", "", "Instrução personalizada de estilo visual")
	imageGenerateCmd.Flags().StringVarP(&genOutputDir, "output", "o", "", "Diretório onde as imagens geradas serão salvas")
	imageGenerateCmd.Flags().IntVarP(&genWorkers, "workers", "w", 0, "Número de workers simultâneos (padrão: 0 para adaptativo)")
	imageGenerateCmd.Flags().BoolVar(&genPreview, "preview", true, "Renderiza miniaturas ANSI TrueColor no terminal conforme cada imagem é gerada")
	imageGenerateCmd.Flags().BoolVar(&genCards, "cards", true, "Gera layout HTML A4 de fichas com legendas pronto para impressão")
	imageGenerateCmd.Flags().BoolVar(&gen2UpPDF, "2up", false, "Compila automaticamente todas as imagens geradas em um PDF 2-up A4")
	imageGenerateCmd.Flags().BoolVarP(&genVerbose, "verbose", "v", false, "Exibe logs detalhados de depuração da API")
	imageGenerateCmd.Flags().StringVarP(&genModelName, "model", "m", ai.DefaultModel, "Modelo de IA para geração de imagens")
	imageGenerateCmd.Flags().StringVar(&genTextModel, "text-model", ai.DefaultTextModel, "Modelo de IA para síntese de prompts")

	imageCmd.AddCommand(imageGenerateCmd)
	RootCmd.AddCommand(imageGenerateCmd)
}
