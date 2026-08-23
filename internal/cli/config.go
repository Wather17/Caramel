package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Gerencia as configurações e chaves de API do Caramel",
	Long: `Permite definir, visualizar e configurar interativamente as chaves de API e preferências salvas no .env do usuário.

📚 QUANDO USAR:
Use no primeiro uso do Caramel para cadastrar sua chave de API do OpenRouter (necessária para os
comandos com IA: generate, colorize, process, docx extract e routine). As credenciais ficam
salvas com segurança em ~/.config/caramel/.env (ou %APPDATA% no Windows).

EXEMPLOS:
# Assistente interativo de configuração
caramel config setup

# Definir a chave manualmente
caramel config set openrouter_key sk-or-v1-suachaveaqui

# Verificar o status das configurações
caramel config show`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <CHAVE> <VALOR>",
	Short: "Define o valor de uma chave de configuração (ex: openrouter_key)",
	Long: `Define manualmente o valor de uma chave de configuração sem passar pelo assistente interativo.

📚 QUANDO USAR:
Use para cadastrar ou trocar a chave de API do OpenRouter de forma direta e automatizada
(ex: em scripts). O valor é salvo no arquivo .env do usuário.`,
	Example: `# Definir a chave de API do OpenRouter
caramel config set openrouter_key sk-or-v1-suachaveaqui`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToUpper(strings.ReplaceAll(args[0], "-", "_"))
		val := args[1]

		switch key {
		case "OPENROUTER_KEY", "OPENROUTER":
			key = "OPENROUTER_API_KEY"
		case "MODEL_IMAGE", "IMAGEMODEL", "IMAGE_MODEL":
			key = "MODEL_IMAGE"
		case "MODEL_TEXT", "TEXTMODEL", "TEXT_MODEL":
			key = "MODEL_TEXT"
		case "MODEL_TRIAGE", "TRIAGEMODEL", "TRIAGE_MODEL":
			key = "MODEL_TRIAGE"
		}

		if err := config.SaveConfigValue(key, val); err != nil {
			return err
		}

		envPath, _ := config.GetEnvFilePath()
		fmt.Printf("✅ Configuração '%s' salva com sucesso em: %s\n", key, envPath)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Exibe a localização do arquivo de configuração e o status das chaves",
	Long: `Exibe a localização do arquivo de configuração (.env) e o status das chaves cadastradas.

📚 QUANDO USAR:
Use para verificar se a chave do OpenRouter está ativa e onde o arquivo de configuração está
salvo no seu sistema — útil para diagnóstico antes de usar comandos com IA.`,
	Example: `# Verificar o status atual das configurações
caramel config show`,
	RunE: func(cmd *cobra.Command, args []string) error {
		envPath, err := config.GetEnvFilePath()
		if err != nil {
			return err
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		fmt.Printf("⚙️  Arquivo de Configuração: %s\n", envPath)
		if cfg.OpenRouterAPIKey == "" {
			fmt.Println(" └─ OPENROUTER_API_KEY: ❌ Não configurada (use 'caramel config setup' ou 'caramel config set openrouter_key <chave>')")
		} else {
			fmt.Printf(" └─ OPENROUTER_API_KEY: ✅ Configurada (%s)\n", obfuscateKey(cfg.OpenRouterAPIKey))
		}

		fmt.Println(" 🤖 Modelos de IA (configuráveis com 'caramel config models'):")
		fmt.Printf("    ├─ MODEL_IMAGE:  %s\n", describeModel(cfg.ModelImage, ai.DefaultModel))
		fmt.Printf("    ├─ MODEL_TEXT:   %s\n", describeModel(cfg.ModelText, ai.DefaultTextModel))
		fmt.Printf("    └─ MODEL_TRIAGE: %s\n", describeModel(cfg.ModelTriage, ai.DefaultTriageModel))
		return nil
	},
}

var configSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Assistente interativo de configuração de chaves de API",
	Long: `Assistente passo a passo para cadastrar a chave de API do OpenRouter, salvando as
credenciais com segurança no arquivo .env do usuário.

📚 QUANDO USAR:
Use no primeiro uso do Caramel para configurar sua chave de API do OpenRouter de forma guiada,
sem precisar editar arquivos manualmente.`,
	Example: `# Iniciar o assistente guiado de configuração
caramel config setup`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("🍬 Assistente de Configuração do Caramel CLI")
		fmt.Println("============================================")
		fmt.Print("Informe a sua chave de API do OpenRouter (ou Pressione Enter para ignorar): ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		key := strings.TrimSpace(input)
		if key != "" {
			if err := config.SaveConfigValue("OPENROUTER_API_KEY", key); err != nil {
				return err
			}
			fmt.Println("✅ Chave OPENROUTER_API_KEY salva com sucesso!")
		} else {
			fmt.Println("ℹ️  Nenhuma chave foi informada.")
		}

		return nil
	},
}

// obfuscateKey oculta parte da chave de API para exibição segura no terminal
func obfuscateKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	prefix := key[:6]
	suffix := key[len(key)-4:]
	return fmt.Sprintf("%s...%s", prefix, suffix)
}

// describeModel formata o status de um modelo para o 'config show'
func describeModel(cfgVal, factory string) string {
	if cfgVal != "" {
		return fmt.Sprintf("✅ %s (configurado)", cfgVal)
	}
	return fmt.Sprintf("❌ não configurado — usando padrão: %s", factory)
}

var (
	configModelsList  bool
	configModelsRole  string
	configModelsLimit int
	configModelsQuery string
)

var configModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Lista e escolhe os modelos de IA da OpenRouter (imagem, texto e triagem)",
	Long: `Consulta o catálogo público de modelos da OpenRouter e abre uma TUI dividida em
três categorias — imagem, texto e triagem — com busca incremental (digite para filtrar).
Ao confirmar, salva as escolhas em MODEL_IMAGE, MODEL_TEXT e MODEL_TRIAGE no .env.

📚 QUANDO USAR:
Use para trocar os modelos padrão do Caramel sem digitar a flag -m toda vez. A prioridade
de resolução é: flag no comando > valor salvo no .env > padrão de fábrica.
Use '--list' para imprimir os modelos em texto puro (útil para scripts).`,
	Example: `# Abrir a TUI de seleção de modelos (imagem, texto e triagem)
caramel config models

# Listar modelos de imagem em texto puro
caramel config models --list --role image --limit 10

# Listar modelos de texto que contenham 'deepseek'
caramel config models --list --role text --search deepseek`,
	RunE: func(cmd *cobra.Command, args []string) error {
		models, err := ai.ListModels()
		if err != nil {
			return fmt.Errorf("não foi possível consultar os modelos da OpenRouter: %w", err)
		}

		if configModelsList {
			return listModelsPlain(models)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		imageModels := ai.FilterModelsByRole(models, ai.RoleImage)
		textModels := ai.FilterModelsByRole(models, ai.RoleText)
		triageModels := ai.FilterModelsByRole(models, ai.RoleTriage)

		// Remove roteadores automáticos (preço -1) e ordena por preço
		imageModels = ai.PricedModels(imageModels)
		textModels = ai.PricedModels(textModels)
		triageModels = ai.PricedModels(triageModels)

		ai.SortModelsByPrice(imageModels)
		ai.SortModelsByPrice(textModels)
		ai.SortModelsByPrice(triageModels)

		defaults := ui.ModelDefaults{
			ImageModel:  orDefault(cfg.ModelImage, ai.DefaultModel),
			TextModel:   orDefault(cfg.ModelText, ai.DefaultTextModel),
			TriageModel: orDefault(cfg.ModelTriage, ai.DefaultTriageModel),
		}

		sel, err := ui.SelectModelsInteractive(imageModels, textModels, triageModels, defaults)
		if err != nil {
			return err
		}

		saved := 0
		if sel.ImageModel != "" && sel.ImageModel != cfg.ModelImage {
			if err := config.SaveConfigValue("MODEL_IMAGE", sel.ImageModel); err != nil {
				return err
			}
			saved++
		}
		if sel.TextModel != "" && sel.TextModel != cfg.ModelText {
			if err := config.SaveConfigValue("MODEL_TEXT", sel.TextModel); err != nil {
				return err
			}
			saved++
		}
		if sel.TriageModel != "" && sel.TriageModel != cfg.ModelTriage {
			if err := config.SaveConfigValue("MODEL_TRIAGE", sel.TriageModel); err != nil {
				return err
			}
			saved++
		}

		if saved == 0 {
			fmt.Println("ℹ️  Nenhuma alteração de modelo foi feita.")
		} else {
			envPath, _ := config.GetEnvFilePath()
			fmt.Printf("✅ %d modelo(s) atualizado(s) e salvo(s) em: %s\n", saved, envPath)
			fmt.Printf("    ├─ MODEL_IMAGE:  %s\n", orDefault(sel.ImageModel, ai.DefaultModel))
			fmt.Printf("    ├─ MODEL_TEXT:   %s\n", orDefault(sel.TextModel, ai.DefaultTextModel))
			fmt.Printf("    └─ MODEL_TRIAGE: %s\n", orDefault(sel.TriageModel, ai.DefaultTriageModel))
		}
		return nil
	},
}

// listModelsPlain imprime os modelos do catálogo em texto puro, sem TUI
func listModelsPlain(models []ai.Model) error {
	role := configModelsRole
	if role == "" {
		role = ai.RoleImage
	}

	filtered := ai.FilterModelsByRole(models, role)
	filtered = ai.PricedModels(filtered)
	ai.SortModelsByPrice(filtered)
	if configModelsQuery != "" {
		filtered = ai.SearchModels(filtered, configModelsQuery)
	}

	limit := configModelsLimit
	if limit <= 0 || limit > len(filtered) {
		limit = len(filtered)
	}

	var titles = map[string]string{
		ai.RoleImage:  "🎨 Modelos de Imagem",
		ai.RoleTriage: "🔍 Modelos de Triagem (Visão)",
		ai.RoleText:   "🧠 Modelos de Texto",
	}
	fmt.Printf("%s (%d encontrados, exibindo %d):\n", titles[role], len(filtered), limit)

	for i, m := range filtered[:limit] {
		fmt.Printf("  %2d. %-55s · %-45s · $%.2f/M\n", i+1, m.ID, truncateModelName(m.Name, 45), m.PromptPrice*1e6)
	}
	return nil
}

// orDefault retorna o valor configurado ou o fallback
func orDefault(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

// truncateModelName limita o nome a uma largura máxima de caracteres
func truncateModelName(name string, max int) string {
	runes := []rune(name)
	if len(runes) <= max {
		return name
	}
	return string(runes[:max-1]) + "…"
}

func init() {
	configModelsCmd.Flags().BoolVar(&configModelsList, "list", false, "Imprime os modelos em texto puro em vez de abrir a TUI")
	configModelsCmd.Flags().StringVar(&configModelsRole, "role", "", "Filtra por papel no modo --list: image, text ou triage (padrão: image)")
	configModelsCmd.Flags().IntVar(&configModelsLimit, "limit", 15, "Quantidade de modelos a exibir no modo --list")
	configModelsCmd.Flags().StringVar(&configModelsQuery, "search", "", "Busca por termo no id/nome no modo --list")

	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetupCmd)
	configCmd.AddCommand(configModelsCmd)
	RootCmd.AddCommand(configCmd)
}
