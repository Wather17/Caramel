package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/output"
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
	Use:    "extract <arquivo.docx>",
	Hidden: true,
	Short:  "Extrai, lista ou colore imagens contidas em um arquivo .docx",
	Long: `Inspeciona a estrutura interna do arquivo .docx fornecido e extrai todas as imagens 
(diagramas, fotos, gráficos) encontradas na pasta 'word/media/' para um diretório especificado.
Com a flag --colorize (-c), as imagens em preto e branco são coloridas automaticamente via IA (OpenRouter).
Com a flag --interactive (-i), exibe um menu interativo para escolher quais imagens extrair/colorir.

📚 QUANDO USAR:
Use quando precisar apenas extrair as figuras de um documento Word para reaproveitá-las em outro
material (apresentações, provas ou atividades no Google Classroom). Use --list para inspecionar
o conteúdo sem extrair nada para o disco. Ao colorir, a triagem de economia pula automaticamente
imagens já coloridas, textos, tabelas e logos.`,
	Example: `# Apenas listar as imagens contidas na prova de Geografia
caramel docx extract prova_geografia.docx --list

# Extrair todas as imagens para uma pasta específica
caramel docx extract atividade.docx -o ./imagens_atividade

# Extrair e colorir as figuras via IA
caramel docx extract mapa_biologia.docx -c`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
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
				return renderer.Result(output.Result{Status: output.StateWarning, Summary: fmt.Sprintf("Nenhuma imagem foi encontrada em %s.", filepath.Base(docxPath))})
			}

			if renderer.Options().JSON {
				return renderer.Result(output.Result{Status: output.StateSuccess, Summary: fmt.Sprintf("%d imagem(ns) encontrada(s).", len(images)), Count: len(images), Data: images})
			}

			if err := renderer.Text("🔍 Imagens encontradas em '%s':\n", filepath.Base(docxPath)); err != nil {
				return err
			}
			for i, img := range images {
				sizeKB := float64(img.Size) / 1024.0
				if err := renderer.Text("  %d. %s (%s, %.1f KB)\n", i+1, img.OriginalName, strings.ToUpper(img.Format), sizeKB); err != nil {
					return err
				}
			}
			return renderer.Text("\nTotal: %d imagem(ns) encontrada(s).\n", len(images))
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

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		// Resolve os modelos com prioridade: flag > config (.env) > default de fábrica
		modelName := resolveModel(modelName, cmd.Flags().Changed("model"), cfg.ModelImage)
		triageModel := resolveModel(docxTriageModel, cmd.Flags().Changed("triage-model"), cfg.ModelTriage)

		// Se a flag --interactive (-i) estiver ativada, exibe a TUI de seleção de imagens
		if docxInteractive {
			allImages, err := docx.ListImages(docxPath)
			if err != nil {
				return err
			}

			if len(allImages) == 0 {
				return renderer.Result(output.Result{Status: output.StateWarning, Summary: fmt.Sprintf("Nenhuma imagem foi encontrada em %s.", filepath.Base(docxPath))})
			}

			keptImages, skippedImages := docx.FilterImagesByMinSize(allImages, minSizeBytes)
			if len(keptImages) == 0 {
				return renderer.Result(output.Result{
					Status:  output.StateWarning,
					Summary: fmt.Sprintf("Nenhuma imagem atende ao tamanho mínimo de %s.", minSizeStr),
					Count:   len(skippedImages),
				})
			}

			selectedImages, err := ui.SelectImagesInteractive(keptImages)
			if err != nil {
				return err
			}

			if len(selectedImages) == 0 {
				return renderer.Result(output.Result{Status: output.StateCanceled, Summary: "Nenhuma imagem foi selecionada."})
			}

			if colorize {
				if cfg.OpenRouterAPIKey == "" {
					return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
				}

				pipeRes, err := pipeline.RunDocxPipelineSelectedWithOptions(docxPath, targetDir, cfg.OpenRouterAPIKey, modelName, selectedImages, pipeline.PipelineOptions{Verbose: outputOptions().Verbose, DiagnosticWriter: cmd.ErrOrStderr()}, triageModel, docxNoTriage)
				if err != nil {
					return err
				}

				return renderDocxPipelineResult(renderer, pipeRes, true)
			}

			res, err := docx.ExtractImagesFromList(docxPath, targetDir, selectedImages)
			if err != nil {
				return err
			}

			return renderDocxExtractionResult(renderer, res, skippedImages, minSizeStr)
		}

		// Se a flag --colorize (-c) foi ativada
		if colorize {
			if cfg.OpenRouterAPIKey == "" {
				return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>' para poder utilizar a IA de coloração")
			}

			pipeRes, err := pipeline.RunDocxPipelineWithOptions(docxPath, targetDir, cfg.OpenRouterAPIKey, modelName, minSizeBytes, pipeline.PipelineOptions{Verbose: outputOptions().Verbose, DiagnosticWriter: cmd.ErrOrStderr()}, triageModel, docxNoTriage)
			if err != nil {
				return err
			}
			return renderDocxPipelineResult(renderer, pipeRes, false)
		}

		// Processo de extração padrão (sem coloração)
		res, err := docx.ExtractImagesFiltered(docxPath, targetDir, minSizeBytes)
		if err != nil {
			return err
		}
		return renderDocxExtractionResult(renderer, res, res.SkippedImages, minSizeStr)
	},
}

func renderDocxExtractionResult(renderer *output.Renderer, res *docx.ExtractionResult, skipped []docx.ExtractedImage, minSizeStr string) error {
	if res == nil {
		return fmt.Errorf("extração DOCX não retornou resultado")
	}

	warnings := make([]string, 0, 1)
	if len(skipped) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d imagem(ns) ignorada(s) por serem menores que %s", len(skipped), minSizeStr))
		if renderer.Options().Verbose {
			for _, img := range skipped {
				sizeKB := float64(img.Size) / 1024.0
				renderer.Diagnostic("⏭️ filtro: %s (%.1f KB)\n", img.OriginalName, sizeKB)
			}
		}
	}

	status := output.StateSuccess
	summary := fmt.Sprintf("Extração concluída: %d imagem(ns) salva(s).", res.TotalExtracted)
	if res.TotalExtracted == 0 {
		status = output.StateWarning
		summary = fmt.Sprintf("Nenhuma imagem atende ao tamanho mínimo de %s.", minSizeStr)
	}

	return renderer.Result(output.Result{
		Status:   status,
		Summary:  summary,
		Count:    res.TotalExtracted,
		Data:     res.Images,
		Outputs:  []string{res.OutputDir},
		Warnings: warnings,
	})
}

var docxImagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Lista e extrai imagens de documentos .docx",
	Long: `Organiza as operações de inspeção e extração de imagens contidas em documentos Word.

📚 QUANDO USAR:
Use este grupo quando precisar localizar figuras em um .docx ou salvá-las para reutilização.
Para colorir imagens, use o comando separado 'caramel image colorize'.`,
}

var docxImagesListCmd = &cobra.Command{
	Use:   "list <arquivo.docx>",
	Short: "Lista imagens contidas em um arquivo .docx",
	Long: `Inspeciona o arquivo .docx e lista as imagens encontradas em 'word/media/'.

📚 QUANDO USAR:
Use antes da extração para conferir nomes, formatos e tamanhos das imagens do documento.`,
	Example: `# Conferir as imagens de um documento Word
caramel docx images list prova_geografia.docx`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDocxExtractCompatibility(cmd, args, true)
	},
}

var docxImagesExtractCmd = &cobra.Command{
	Use:   "extract <arquivo.docx>",
	Short: "Extrai imagens contidas em um arquivo .docx",
	Long: `Extrai as imagens encontradas em 'word/media/' para um diretório de destino.

📚 QUANDO USAR:
Use para reaproveitar figuras de um documento Word em apresentações, provas ou atividades.
Use --output para escolher a pasta de destino e --min-size para filtrar imagens pequenas.`,
	Example: `# Extrair imagens para uma pasta específica
caramel docx images extract atividade.docx --output ./imagens_atividade`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDocxExtractCompatibility(cmd, args, false)
	},
}

// runDocxExtractCompatibility mantém a implementação legada como compatibilidade
// enquanto os caminhos específicos da árvore canônica reutilizam o mesmo handler.
func runDocxExtractCompatibility(cmd *cobra.Command, args []string, list bool) error {
	previousList, previousColorize := listOnly, colorize
	defer func() {
		listOnly, colorize = previousList, previousColorize
	}()
	listOnly = list
	colorize = false
	return docxExtractCmd.RunE(cmd, args)
}

func addDocxImagesExtractFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Diretório onde as imagens serão salvas")
	cmd.Flags().StringVarP(&minSizeStr, "min-size", "s", "0", "Tamanho mínimo da imagem para ser extraída")
	cmd.Flags().BoolVarP(&docxInteractive, "interactive", "i", false, "Habilita seleção interativa das imagens")
}

func init() {
	// Flags do comando extract
	docxExtractCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Diretório onde as imagens serão salvas (padrão: imagens <nome_do_arquivo>)")
	docxExtractCmd.Flags().BoolVarP(&listOnly, "list", "l", false, "Apenas lista as imagens encontradas sem extraí-las para o disco")
	docxExtractCmd.Flags().BoolVarP(&colorize, "colorize", "c", false, "Colora automaticamente as imagens extraídas via IA (OpenRouter)")
	docxExtractCmd.Flags().StringVarP(&modelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração (config: model_image)")
	docxExtractCmd.Flags().StringVarP(&minSizeStr, "min-size", "s", "0", "Tamanho mínimo da imagem para ser extraída (ex: '20KB', '50KB', '0' para todas)")
	docxExtractCmd.Flags().BoolVarP(&docxInteractive, "interactive", "i", false, "Exibe menu interativo para selecionar quais imagens extrair/processar")
	docxExtractCmd.Flags().StringVar(&docxTriageModel, "triage-model", ai.DefaultTriageModel, "Modelo de IA de visão usado na triagem de economia antes da coloração (config: model_triage)")
	docxExtractCmd.Flags().BoolVar(&docxNoTriage, "no-triage", false, "Desativa a triagem e colora todas as imagens elegíveis diretamente")

	// Registra subcomandos
	docxCmd.AddCommand(docxExtractCmd)
	addDocxImagesExtractFlags(docxImagesExtractCmd)
	docxImagesCmd.AddCommand(docxImagesListCmd)
	docxImagesCmd.AddCommand(docxImagesExtractCmd)
	docxCmd.AddCommand(docxImagesCmd)
	RootCmd.AddCommand(docxCmd)
}
