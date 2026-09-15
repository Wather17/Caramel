package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"caramel/internal/config"
	"caramel/internal/output"
	"caramel/internal/prompts"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"

	"github.com/spf13/cobra"
)

var (
	routineOutputDir string
	routineModelName string
	routinePromptDir string
	routineWorkers   int
)

type routineFileResult struct {
	rows    []docx.RoutineRow
	skipped bool
	err     error
	detail  string
}

var routineCmd = &cobra.Command{
	Use:   "routine",
	Short: "Ferramentas para processamento e organização de rotinas pedagógicas",
	Long:  `Grupo de comandos dedicados à extração, síntese via IA e compilação de rotinas de aula.`,
}

var routineProcessCmd = &cobra.Command{
	Use:    "process <pasta_ou_arquivo.docx>",
	Hidden: true,
	Short:  "Processa rotinas de aula (.docx), extrai dados via IA e gera o documento final consolidado",
	Long: `Inspeciona arquivos .docx com as rotinas semanais de aula, extrai as informações de texto,
envia ao OpenRouter para resumir e classificar os Campos de Experiência da BNCC,
e compila tudo cronologicamente em um único arquivo .docx formatado em Paisagem.

📚 QUANDO USAR:
Use para transformar rotinas pedagógicas semanais (uma pasta ou um arquivo .docx) em um relatório
consolidado com as experiências classificadas de acordo com os Campos de Experiência da BNCC,
pronto para o planejamento do professor.`,
	Example: `# Processar todas as rotinas semanais de uma pasta e gerar o relatório consolidado
caramel routine process ./abril/

# Processar uma rotina única de uma semana específica
caramel routine process rotina_semana_1.docx`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		targetPath := args[0]

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup'")
		}

		// Resolve o modelo com prioridade: flag > config (.env) > default de fábrica
		routineModel := resolveModel(routineModelName, cmd.Flags().Changed("model"), cfg.ModelText)

		// 1. Coleta os arquivos .docx a serem processados
		var files []string
		fileInfo, err := os.Stat(targetPath)
		if err != nil {
			return fmt.Errorf("caminho inválido: %w", err)
		}

		if fileInfo.IsDir() {
			entries, err := os.ReadDir(targetPath)
			if err != nil {
				return fmt.Errorf("erro ao ler diretório: %w", err)
			}
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".docx") {
					// Ignora arquivos temporários do Word que começam com ~$
					if !strings.HasPrefix(entry.Name(), "~$") {
						files = append(files, filepath.Join(targetPath, entry.Name()))
					}
				}
			}
		} else {
			if strings.HasSuffix(strings.ToLower(targetPath), ".docx") {
				files = append(files, targetPath)
			} else {
				return fmt.Errorf("o arquivo fornecido não é um documento .docx")
			}
		}

		if len(files) == 0 {
			return renderer.Result(output.Result{Status: output.StateWarning, Summary: "Nenhuma rotina .docx encontrada para processar."})
		}

		renderer.Diagnostic("processando %d rotina(s)\n", len(files))

		// 2. Carrega o prompt de análise de rotinas
		var prompt string
		if routinePromptDir != "" {
			data, err := os.ReadFile(routinePromptDir)
			if err != nil {
				return fmt.Errorf("erro ao carregar prompt customizado: %w", err)
			}
			prompt = strings.TrimSpace(string(data))
		} else {
			prompt = prompts.GetRoutinePrompt()
		}

		aiClient, err := ai.NewClient(cfg.OpenRouterAPIKey)
		if err != nil {
			return err
		}
		aiClient.Verbose = outputOptions().Verbose
		aiClient.DiagnosticWriter = cmd.ErrOrStderr()

		var combinedRows []docx.RoutineRow
		processed := 0
		skipped := 0
		failed := 0

		// 3. Processa cada arquivo como uma unidade independente. A coleta dos
		// resultados permanece indexada para que a saída e os diagnósticos sigam
		// a ordem determinística dos arquivos, mesmo com workers concorrentes.
		sort.Strings(files)
		fileResults := make([]routineFileResult, len(files))
		itemErrors, batchErr := ai.ExecuteBatchContext(cmd.Context(), len(files), routineWorkers, func(workCtx context.Context, index int) error {
			file := files[index]
			result := &fileResults[index]
			txt, err := docx.ExtractText(file)
			if err != nil {
				result.err = err
				result.detail = fmt.Sprintf("falha ao extrair '%s': %v", filepath.Base(file), err)
				return err
			}
			if err := workCtx.Err(); err != nil {
				result.err = err
				result.detail = fmt.Sprintf("análise cancelada para '%s': %v", filepath.Base(file), err)
				return err
			}

			jsonResponse, err := aiClient.AnalyzeRoutineContext(workCtx, txt, prompt, routineModel)
			if err != nil {
				result.err = err
				result.detail = fmt.Sprintf("falha na análise de '%s': %v", filepath.Base(file), err)
				return err
			}

			var fileRows []docx.RoutineRow
			if err := json.Unmarshal([]byte(jsonResponse), &fileRows); err != nil {
				result.err = err
				result.detail = fmt.Sprintf("falha ao decodificar JSON de '%s': %v; resposta raw: %s", filepath.Base(file), err, jsonResponse)
				return err
			}
			if len(fileRows) == 0 {
				result.skipped = true
				return nil
			}
			result.rows = fileRows
			return nil
		}, nil)
		if batchErr != nil {
			return batchErr
		}
		for index, err := range itemErrors {
			if fileResults[index].err == nil && err != nil {
				fileResults[index].err = err
				fileResults[index].detail = fmt.Sprintf("falha ao processar '%s': %v", filepath.Base(files[index]), err)
			}
		}
		for index, result := range fileResults {
			file := files[index]
			if result.err != nil {
				failed++
				renderer.Diagnostic("%s\n", result.detail)
				continue
			}
			if result.skipped {
				skipped++
				continue
			}
			processed++
			combinedRows = append(combinedRows, result.rows...)
			renderer.Diagnostic("rotina processada: %s\n", filepath.Base(file))
		}

		if len(combinedRows) == 0 {
			return fmt.Errorf("nenhum dado válido pôde ser extraído e compilado pela IA (processadas: %d; puladas: %d; falhas: %d)", processed, skipped, failed)
		}

		// 4. Ordena os registros cronologicamente por data usando parsing resiliente
		sort.SliceStable(combinedRows, func(i, j int) bool {
			t1 := parseResilientDate(combinedRows[i].Data)
			t2 := parseResilientDate(combinedRows[j].Data)
			if !t1.IsZero() && !t2.IsZero() {
				return t1.Before(t2)
			}
			// Fallback para ordenação alfabética
			return combinedRows[i].Data < combinedRows[j].Data
		})

		// 5. Gera o arquivo de destino
		targetOutDir := routineOutputDir
		if targetOutDir == "" {
			if fileInfo.IsDir() {
				targetOutDir = targetPath
			} else {
				targetOutDir = filepath.Dir(targetPath)
			}
		}

		finalDocxName := fmt.Sprintf("Campos_de_experiências_%s.docx", time.Now().Format("02-01-2006"))
		finalDocxPath := filepath.Join(targetOutDir, finalDocxName)

		renderer.Diagnostic("gerando relatório final em paisagem\n")
		docxBytes, err := docx.GeneratePedagogicalReport(combinedRows)
		if err != nil {
			return fmt.Errorf("erro ao gerar documento Word consolidado: %w", err)
		}

		if err := os.WriteFile(finalDocxPath, docxBytes, 0644); err != nil {
			return fmt.Errorf("erro ao gravar arquivo final no disco: %w", err)
		}

		status := output.StateSuccess
		warnings := []string{}
		if skipped > 0 {
			status = output.StateWarning
			warnings = append(warnings, fmt.Sprintf("%d rotina(s) sem dados válidos", skipped))
		}
		if failed > 0 {
			status = output.StateWarning
			warnings = append(warnings, fmt.Sprintf("%d rotina(s) falharam durante o processamento", failed))
		}
		return renderer.Result(output.Result{
			Status:   status,
			Summary:  fmt.Sprintf("Rotinas consolidadas: %d processada(s); %d pulada(s); %d falha(s).", processed, skipped, failed),
			Count:    processed,
			Outputs:  []string{finalDocxPath},
			Warnings: warnings,
		})
	},
}

var routineConsolidateCmd = &cobra.Command{
	Use:   "consolidate <pasta_ou_arquivo.docx>",
	Short: "Consolida rotinas de aula em um relatório .docx",
	Long: `Lê uma ou mais rotinas semanais, resume as atividades com IA e gera um relatório consolidado
classificado pelos Campos de Experiência da BNCC.

📚 QUANDO USAR:
Use para reunir rotinas de aula em um único documento pronto para revisão e planejamento pedagógico.`,
	Example: `# Consolidar todas as rotinas de uma pasta
caramel routine consolidate ./abril/`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return routineProcessCmd.RunE(cmd, args)
	},
}

func addRoutineFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&routineOutputDir, "output", "o", "", "Diretório de destino do relatório consolidado")
	cmd.Flags().StringVarP(&routineModelName, "model", "m", ai.DefaultTextModel, "Modelo de IA para análise")
	cmd.Flags().StringVarP(&routinePromptDir, "prompt", "p", "", "Arquivo com prompt personalizado")
	cmd.Flags().IntVarP(&routineWorkers, "workers", "w", 0, "Número de workers simultâneos (padrão: 0 para adaptativo)")
}

func init() {
	routineProcessCmd.Flags().StringVarP(&routineOutputDir, "output", "o", "", "Diretório de destino para salvar o arquivo consolidado")
	routineProcessCmd.Flags().StringVarP(&routineModelName, "model", "m", ai.DefaultTextModel, "Modelo de IA do OpenRouter para a análise (config: model_text)")
	routineProcessCmd.Flags().StringVarP(&routinePromptDir, "prompt", "p", "", "Caminho para arquivo contendo prompt customizado")
	routineProcessCmd.Flags().IntVarP(&routineWorkers, "workers", "w", 0, "Número de workers simultâneos (padrão: 0 para adaptativo)")
	addRoutineFlags(routineConsolidateCmd)

	routineCmd.AddCommand(routineProcessCmd)
	routineCmd.AddCommand(routineConsolidateCmd)
	RootCmd.AddCommand(routineCmd)
}

// parseResilientDate attempts to parse date strings in various formats, sanitizing "YY" placeholders
func parseResilientDate(dateStr string) time.Time {
	d := strings.TrimSpace(dateStr)

	// Normalize standard placeholder "YY" or "yy" to "26" (current school year)
	d = strings.ReplaceAll(d, "/YY", "/26")
	d = strings.ReplaceAll(d, "/yy", "/26")

	// Formats generated by LLMs or in documents
	formats := []string{
		"02/01/06",   // e.g. 30/03/26
		"02/01/2006", // e.g. 30/03/2026
		"02/01",      // e.g. 06/04 (no year)
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, d); err == nil {
			// If format has no year, assume 2026
			if layout == "02/01" {
				return time.Date(2026, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			}
			return t
		}
	}
	return time.Time{}
}
