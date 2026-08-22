package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"caramel/internal/config"
	"caramel/internal/prompts"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"

	"github.com/spf13/cobra"
)

var (
	routineOutputDir string
	routineModelName string
	routinePromptDir string
	routineVerbose   bool
)

var routineCmd = &cobra.Command{
	Use:   "routine",
	Short: "Ferramentas para processamento e organização de rotinas pedagógicas",
	Long:  `Grupo de comandos dedicados à extração, síntese via IA e compilação de rotinas de aula.`,
}

var routineProcessCmd = &cobra.Command{
	Use:     "process <pasta_ou_arquivo.docx>",
	Aliases: []string{"pipeline", "run"},
	Short:   "Processa rotinas de aula (.docx), extrai dados via IA e gera o documento final consolidado",
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
		targetPath := args[0]

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.OpenRouterAPIKey == "" {
			return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup'")
		}

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
			fmt.Println("ℹ️ Nenhuma rotina .docx encontrada para processar.")
			return nil
		}

		fmt.Printf("🚀 Iniciando processamento de %d rotina(s)...\n", len(files))

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
		aiClient.Verbose = routineVerbose

		var combinedRows []docx.RoutineRow

		// 3. Processa cada arquivo
		for _, file := range files {
			fmt.Printf(" ├─ Lendo texto de: %s\n", filepath.Base(file))
			txt, err := docx.ExtractText(file)
			if err != nil {
				fmt.Printf(" ⚠️ Erro ao extrair texto de '%s': %v. Pulando...\n", filepath.Base(file), err)
				continue
			}

			fmt.Printf(" │  └─ Consultando IA para resumir atividades...\n")
			jsonResponse, err := aiClient.AnalyzeRoutine(txt, prompt, routineModelName)
			if err != nil {
				fmt.Printf(" ⚠️ Erro na análise de IA para '%s': %v. Pulando...\n", filepath.Base(file), err)
				continue
			}

			// Parse do JSON parcial de cada arquivo
			var fileRows []docx.RoutineRow
			if err := json.Unmarshal([]byte(jsonResponse), &fileRows); err != nil {
				// Tenta decodificar o erro ou printa resposta bruta
				fmt.Printf(" ⚠️ Falha ao decodificar JSON retornado pela IA para '%s'. Resposta raw: %s\n", filepath.Base(file), jsonResponse)
				continue
			}

			combinedRows = append(combinedRows, fileRows...)
		}

		if len(combinedRows) == 0 {
			return fmt.Errorf("nenhum dado válido pôde ser extraído e compilado pela IA")
		}

		// 4. Ordena os registros cronologicamente por data usando parsing resiliente
		sort.Slice(combinedRows, func(i, j int) bool {
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

		fmt.Printf(" ├─ Gerando relatório final em Paisagem...\n")
		docxBytes, err := docx.GeneratePedagogicalReport(combinedRows)
		if err != nil {
			return fmt.Errorf("erro ao gerar documento Word consolidado: %w", err)
		}

		if err := os.WriteFile(finalDocxPath, docxBytes, 0644); err != nil {
			return fmt.Errorf("erro ao gravar arquivo final no disco: %w", err)
		}

		fmt.Printf("✅ Sucesso! Relatório pedagógico consolidado e salvo em:\n   👉 %s\n", finalDocxPath)
		return nil
	},
}

func init() {
	routineProcessCmd.Flags().StringVarP(&routineOutputDir, "output", "o", "", "Diretório de destino para salvar o arquivo consolidado")
	routineProcessCmd.Flags().StringVarP(&routineModelName, "model", "m", ai.DefaultTextModel, "Modelo de IA do OpenRouter para a análise")
	routineProcessCmd.Flags().StringVarP(&routinePromptDir, "prompt", "p", "", "Caminho para arquivo contendo prompt customizado")
	routineProcessCmd.Flags().BoolVarP(&routineVerbose, "verbose", "v", false, "Exibe informações detalhadas de depuração e resposta raw da API")

	routineCmd.AddCommand(routineProcessCmd)
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

