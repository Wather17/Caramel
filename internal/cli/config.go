package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"caramel/internal/config"

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
		key := strings.ToUpper(args[0])
		val := args[1]

		if key == "OPENROUTER_KEY" || key == "OPENROUTER" {
			key = "OPENROUTER_API_KEY"
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

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetupCmd)
	RootCmd.AddCommand(configCmd)
}
