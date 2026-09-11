package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/output"
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
	genModelName   string
	genTextModel   string
	genAspect      string
)

// validAspects lista as proporções aceitas pela flag --aspect (valores normalizados do OpenRouter)
var validAspects = []string{
	"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "4:5", "5:4", "21:9", "auto",
}

// validateAspect retorna erro se a proporção informada não for suportada
func validateAspect(aspect string) error {
	for _, v := range validAspects {
		if aspect == v {
			return nil
		}
	}
	return fmt.Errorf("proporção '%s' inválida. Valores aceitos: %s", aspect, strings.Join(validAspects, ", "))
}

var imageGenerateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen"},
	Short:   "Gera ilustrações e objetos pedagógicos em lote com prompts sintetizados por IA",
	Long: `Harness de geração em lote de imagens pedagógicas.
Permite informar uma lista de palavras (ex: frutas, legumes, animais, ações), um arquivo de texto ou um tema descritivo.
A IA sintetiza e padroniza os prompts automaticamente e um motor em Go com concorrência adaptativa gera as imagens rapidamente.

📚 QUANDO USAR:
Use para criar coleções visuais inteiras (5, 10, 30+ itens) com fundo branco e estilo unificado
(clipart, vector, 3d-cute, coloring ou realistic) para fichas, jogos de memória e atividades.
As imagens são geradas em formato 1:1 por padrão — use --aspect para outras proporções
(ex: 16:9 para slides). Compile tudo em um PDF 2-up com --2up, ou diagrame fichas A4 com 'caramel print cards'.`,
	Example: `# Gerar imagens de frutas tropicais em estilo 3D fofo
caramel image generate --items "abacaxi, manga, maracujá, caju" -s 3d-cute

# Gerar 10 animais da fazenda e compilar em PDF 2-up
caramel image generate --theme "animais da fazenda" -n 10 --2up

# Gerar desenhos para colorir a partir de um arquivo de texto
caramel image generate -f ./itens.txt -s coloring

# Gerar imagens em formato widescreen 16:9 (slides/apresentações)
	caramel image generate --items "sol, nuvem, arco-íris" --aspect 16:9`,
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
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

		// Validação da proporção das imagens
		if err := validateAspect(genAspect); err != nil {
			return err
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
		}

		// Resolve os modelos com prioridade: flag > config (.env) > default de fábrica
		imageModel := resolveModel(genModelName, cmd.Flags().Changed("model"), cfg.ModelImage)
		textModel := resolveModel(genTextModel, cmd.Flags().Changed("text-model"), cfg.ModelText)

		client, err := ai.NewClient(cfg.OpenRouterAPIKey)
		if err != nil {
			return err
		}
		client.Verbose = renderer.Options().Verbose
		client.DiagnosticWriter = cmd.ErrOrStderr()

		harnessCfg := ai.HarnessConfig{
			Items:       rawItems,
			Theme:       genTheme,
			Count:       genCount,
			Style:       genStyle,
			CustomStyle: genCustomStyle,
			OutputDir:   genOutputDir,
			MaxWorkers:  genWorkers,
			TextModel:   textModel,
			ImageModel:  imageModel,
			Aspect:      genAspect,
			Verbose:     renderer.Options().Verbose,
		}

		// Estágio 1: Síntese de prompts
		if genTheme != "" && len(rawItems) == 0 {
			renderer.Diagnostic("sintetizando %d itens para o tema %s (estilo: %s)\n", genCount, genTheme, genStyle)
		} else {
			renderer.Diagnostic("sintetizando prompts para %d item(ns) (estilo: %s)\n", len(rawItems), genStyle)
		}

		items, err := ai.SynthesizePrompts(harnessCfg, client)
		if err != nil {
			return err
		}

		for _, it := range items {
			renderer.Diagnostic("item %02d: %s (%s)\n", it.Index, it.Name, it.Slug)
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
		renderer.Diagnostic("motor de geração: %d worker(s), intervalo %s, saída %s\n", workers, delay, targetDir)

		progressFunc := func(ev ai.HarnessProgressEvent) {
			if ev.CurrentStep == "saved" {
				renderer.Text("[%d/%d] gerado: %s\n", ev.Completed, ev.Total, ev.Item.Name)
				if genPreview && ev.Item.ImagePath != "" && !renderer.Options().JSON && !renderer.Options().Quiet {
					ansiArt, err := ui.RenderImageFileToANSI(ev.Item.ImagePath, 40, 20)
					if err == nil && ansiArt != "" {
						renderer.Text("%s\n", ansiArt)
					}
				}
			} else if ev.CurrentStep == "error" {
				renderer.Text("[%d/%d] falha na geração\n", ev.Completed, ev.Total)
				renderer.Diagnostic("falha ao gerar %s: %s\n", ev.Item.Name, ev.Item.Error)
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

		artifacts := []string{targetDir}
		warnings := []string{}
		if failCount > 0 {
			warnings = append(warnings, fmt.Sprintf("%d imagem(ns) falharam durante a geração", failCount))
		}

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
					warnings = append(warnings, "falha ao gerar fichas HTML")
					renderer.Diagnostic("falha ao gerar fichas HTML: %v\n", err)
				} else {
					artifacts = append(artifacts, htmlOutPath)
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

			if err := pdf.Generate2UpPDF(successfulPaths, pdfOutPath, pdfOpts); err != nil {
				warnings = append(warnings, "falha ao compilar PDF 2-up")
				renderer.Diagnostic("falha ao compilar PDF 2-up: %v\n", err)
			} else {
				artifacts = append(artifacts, pdfOutPath)
			}
		}

		status := output.StateSuccess
		if len(warnings) > 0 || successCount == 0 {
			status = output.StateWarning
		}
		return renderer.Result(output.Result{
			Status:   status,
			Summary:  fmt.Sprintf("Geração concluída: %d gerada(s); %d falha(s).", successCount, failCount),
			Count:    successCount,
			Data:     results,
			Outputs:  artifacts,
			Warnings: warnings,
		})
	},
}

func init() {
	imageGenerateCmd.Flags().StringVarP(&genItemsStr, "items", "i", "", "Lista de itens ou palavras separadas por vírgula (ex: 'maçã, banana, uva')")
	imageGenerateCmd.Flags().StringVarP(&genFilePath, "file", "f", "", "Caminho para arquivo .txt com itens linha por linha")
	imageGenerateCmd.Flags().StringVarP(&genTheme, "theme", "t", "", "Tema descritivo para a IA escolher e gerar os itens automaticamente")
	imageGenerateCmd.Flags().IntVarP(&genCount, "count", "n", 10, "Quantidade de itens a gerar quando utilizado com --theme")
	imageGenerateCmd.Flags().StringVarP(&genStyle, "style", "s", "clipart", "Estilo visual: clipart, vector, 3d-cute, coloring, realistic")
	imageGenerateCmd.Flags().StringVar(&genCustomStyle, "custom-style", "", "Instrução personalizada de estilo visual")
	imageGenerateCmd.Flags().StringVar(&genAspect, "aspect", "1:1", "Proporção das imagens geradas: 1:1, 4:3, 3:4, 16:9, 9:16, 3:2, 2:3, 4:5, 5:4, 21:9 ou auto")
	imageGenerateCmd.Flags().StringVarP(&genOutputDir, "output", "o", "", "Diretório onde as imagens geradas serão salvas")
	imageGenerateCmd.Flags().IntVarP(&genWorkers, "workers", "w", 0, "Número de workers simultâneos (padrão: 0 para adaptativo)")
	imageGenerateCmd.Flags().BoolVar(&genPreview, "preview", true, "Renderiza miniaturas ANSI TrueColor no terminal conforme cada imagem é gerada")
	imageGenerateCmd.Flags().BoolVar(&genCards, "cards", true, "Gera layout HTML A4 de fichas com legendas pronto para impressão")
	imageGenerateCmd.Flags().BoolVar(&gen2UpPDF, "2up", false, "Compila automaticamente todas as imagens geradas em um PDF 2-up A4")
	imageGenerateCmd.Flags().StringVarP(&genModelName, "model", "m", ai.DefaultModel, "Modelo de IA para geração de imagens (config: model_image)")
	imageGenerateCmd.Flags().StringVar(&genTextModel, "text-model", ai.DefaultTextModel, "Modelo de IA para síntese de prompts (config: model_text)")

	// Off-switches: permitem desligar comportamentos ligados por padrão
	imageGenerateCmd.Flags().BoolVar(&genPreview, "no-preview", false, "Não renderiza miniaturas ANSI no terminal durante a geração")
	imageGenerateCmd.Flags().BoolVar(&genCards, "no-cards", false, "Não gera o layout HTML A4 de fichas")
	// Restaura os padrões (pflag sobrescreve a variável ao registrar as negativas)
	genPreview = true
	genCards = true

	imageCmd.AddCommand(imageGenerateCmd)
	RootCmd.AddCommand(imageGenerateCmd)
}
