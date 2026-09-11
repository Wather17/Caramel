package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/output"
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
	Aliases: []string{"color", "colorir", "process", "pipeline", "run"},
	Short:   "Colora imagem(ns) ou documentos .docx em preto e branco usando IA (OpenRouter)",
	Long: `Envia ilustrações em preto e branco para a IA e gera versões coloridas.
Aceita arquivos de imagem individuais (PNG, JPG, WEBP), pastas inteiras ou arquivos .docx.

Ao receber um arquivo .docx:
- Executa o pipeline automatizado: extrai as imagens, colore via IA, ajusta proporções e gera um novo arquivo .docx reconstruído ('<nome> colorida.docx') além de salvar as imagens coloridas;
- Suporta a flag '-i'/'--interactive' para seleção interativa com preview ANSI no terminal.

📚 QUANDO USAR:
Use para colorir ilustrações, fotos ou materiais P&B para tornar as atividades mais atrativas.
Antes de colorir, uma triagem de economia analisa cada imagem em duas camadas: análise local
de saturação (custo zero) e um modelo de visão de baixo custo (Qwen 3.7 Flash). Imagens já
coloridas, textos, tabelas, caça-palavras e atividades de 'colorir' são puladas automaticamente —
mas fichas de exercício com ilustrações coloríveis ao redor são aprovadas.
Use '--no-triage' para desativar a triagem.`,
	Example: `# Colorir uma ilustração isolada
caramel colorize desenho.png

# Processar e reconstruir uma prova .docx com imagens coloridas
caramel colorize avaliacao.docx

# Colorir imagens de um .docx com seleção interativa no terminal
caramel colorize atividade.docx -i`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		inputPath := args[0]

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		// Resolve os modelos com prioridade: flag > config (.env) > default de fábrica
		modelName := resolveModel(imgModelName, cmd.Flags().Changed("model"), cfg.ModelImage)
		triageModel := resolveModel(imgTriageModel, cmd.Flags().Changed("triage-model"), cfg.ModelTriage)

		// Se o alvo for um arquivo .docx, executa o pipeline unificado de DOCX
		if strings.ToLower(filepath.Ext(inputPath)) == ".docx" {
			return RunProcessDocx(ProcessDocxOptions{
				DocxPath:    inputPath,
				OutputDir:   imgOutputDir,
				ModelName:   modelName,
				MinSize:     imgMinSize,
				Interactive: interactiveFlag,
				Verbose:     verboseFlag,
				TriageModel: triageModel,
				NoTriage:    imgNoTriage,
				Output:      outputOptions(),
				Out:         cmd.OutOrStdout(),
				Err:         cmd.ErrOrStderr(),
			})
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

		// Seleciona todas sem formulário interativo quando --all ou --no-triage
		// (a triagem desativada já implica processar tudo automaticamente)
		var selectedImages []string
		if allFlag || imgNoTriage {
			selectedImages = candidateImages
		} else if interactiveFlag || (len(candidateImages) > 1 && !renderer.Options().JSON && !renderer.Options().Quiet) {
			if renderer.Options().JSON || renderer.Options().Quiet {
				return fmt.Errorf("os modos --json e --quiet não podem ser combinados com --interactive")
			}
			selected, err := ui.SelectImageFilesWithPreviewInteractive(candidateImages)
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				return renderer.Result(output.Result{Status: output.StateCanceled, Summary: "Nenhuma imagem foi selecionada."})
			}
			selectedImages = selected
		} else {
			selectedImages = candidateImages
		}

		// Determina diretório final de saída para imagens coloridas
		defaultOutputDir := filepath.Dir(inputPath)

		targetDir := imgOutputDir
		if !cmd.Flags().Changed("output") || targetDir == "" {
			targetDir = defaultOutputDir
		}
		renderer.Diagnostic("colorizando %d imagem(ns) com o modelo %s\n", len(selectedImages), modelName)

		var colorized []ai.ColorizeResult
		warnings := []string{}
		successCount := 0
		skipCount := 0
		failCount := 0

		for i, imgPath := range selectedImages {
			renderer.Text("[%d/%d] colorizando %s\n", i+1, len(selectedImages), filepath.Base(imgPath))
			res, err := ai.ColorizeSingleImage(imgPath, ai.ColorizeOptions{
				OutputDir:        targetDir,
				APIKey:           cfg.OpenRouterAPIKey,
				Model:            modelName,
				TriageModel:      triageModel,
				DisableTriage:    imgNoTriage,
				Verbose:          renderer.Options().Verbose,
				DiagnosticWriter: cmd.ErrOrStderr(),
			})
			if err != nil {
				failCount++
				warnings = append(warnings, fmt.Sprintf("falha ao colorir %s", filepath.Base(imgPath)))
				renderer.Diagnostic("falha ao colorir '%s': %v\n", filepath.Base(imgPath), err)
				continue
			}

			if res.Skipped {
				skipCount++
				renderer.Diagnostic("pulada pela triagem: %s (%s)\n", filepath.Base(imgPath), res.SkipReason)
				continue
			}

			successCount++
			colorized = append(colorized, *res)
		}

		status := output.StateSuccess
		if failCount > 0 || skipCount > 0 {
			status = output.StateWarning
		}
		return renderer.Result(output.Result{
			Status:   status,
			Summary:  fmt.Sprintf("Colorização concluída: %d colorizada(s); %d pulada(s); %d falha(s).", successCount, skipCount, failCount),
			Count:    successCount,
			Data:     colorized,
			Outputs:  []string{targetDir},
			Warnings: warnings,
		})
	},
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp"
}

func init() {
	imageColorizeCmd.Flags().StringVarP(&imgOutputDir, "output", "o", "", "Diretório de destino (padrão: pasta da imagem original ou pasta do docx)")
	imageColorizeCmd.Flags().StringVarP(&imgModelName, "model", "m", ai.DefaultModel, "Modelo de IA do OpenRouter para coloração (config: model_image)")
	imageColorizeCmd.Flags().StringVarP(&imgMinSize, "min-size", "s", "0", "Tamanho mínimo da imagem ao processar .docx (ex: '20KB', '50KB', '0' para todas)")
	imageColorizeCmd.Flags().BoolVarP(&interactiveFlag, "interactive", "i", false, "Habilita seleção interativa e preview TUI no terminal")
	imageColorizeCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Processa todas as imagens automaticamente, sem abrir o formulário de seleção")
	imageColorizeCmd.Flags().StringVar(&imgTriageModel, "triage-model", ai.DefaultTriageModel, "Modelo de IA de visão usado na triagem de economia antes da coloração (config: model_triage)")
	imageColorizeCmd.Flags().BoolVar(&imgNoTriage, "no-triage", false, "Desativa a triagem de economia e processa todas as imagens automaticamente, sem seleção")

	imageCmd.AddCommand(imageColorizeCmd)
	RootCmd.AddCommand(imageCmd)

	// Atalho direto no Root Cmd para aceitar 'caramel colorize <imagem|docx>' (compat)
	RootCmd.AddCommand(imageColorizeCmd)
}
