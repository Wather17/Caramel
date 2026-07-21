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
	Long:  `Permite definir, visualizar e configurar interativamente as chaves de API e preferências salvas no .env do usuário.`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <CHAVE> <VALOR>",
	Short: "Define o valor de uma chave de configuração (ex: openrouter_key)",
	Args:  cobra.ExactArgs(2),
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
