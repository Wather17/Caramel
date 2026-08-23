package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config armazena as configurações e chaves de API da aplicação
type Config struct {
	OpenRouterAPIKey string
	ModelImage       string // Modelo de IA para geração/coloração de imagens (MODEL_IMAGE)
	ModelText        string // Modelo de IA para síntese de texto/prompts (MODEL_TEXT)
	ModelTriage      string // Modelo de IA de visão para a triagem de economia (MODEL_TRIAGE)
}

// GetConfigDir retorna o caminho absoluto da pasta de configuração do usuário no sistema operacional
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("não foi possível obter o diretório pessoal do usuário: %w", err)
	}

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "caramel"), nil
		}
		return filepath.Join(homeDir, ".caramel"), nil
	}

	// Linux / macOS: ~/.config/caramel
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome != "" {
		return filepath.Join(configHome, "caramel"), nil
	}
	return filepath.Join(homeDir, ".config", "caramel"), nil
}

// GetEnvFilePath retorna o caminho completo do arquivo .env de configuração do Caramel
func GetEnvFilePath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".env"), nil
}

// LoadConfig carrega as configurações com prioridade:
// 1. Variáveis de ambiente do sistema (ex: OPENROUTER_API_KEY)
// 2. Arquivo .env local na pasta de execução atual
// 3. Arquivo .env global em ~/.config/caramel/.env
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// 1. Carrega do .env global de instalação
	globalEnvPath, err := GetEnvFilePath()
	if err == nil {
		loadEnvFileToMap(globalEnvPath, cfg)
	}

	// 2. Carrega do .env da pasta atual (se existir)
	loadEnvFileToMap(".env", cfg)

	// 3. Variáveis do SO têm prioridade máxima
	if envVal := os.Getenv("OPENROUTER_API_KEY"); envVal != "" {
		cfg.OpenRouterAPIKey = envVal
	}
	if envVal := os.Getenv("MODEL_IMAGE"); envVal != "" {
		cfg.ModelImage = envVal
	}
	if envVal := os.Getenv("MODEL_TEXT"); envVal != "" {
		cfg.ModelText = envVal
	}
	if envVal := os.Getenv("MODEL_TRIAGE"); envVal != "" {
		cfg.ModelTriage = envVal
	}

	return cfg, nil
}

// SaveConfigValue salva ou atualiza uma chave e valor no arquivo .env global do usuário (~/.config/caramel/.env)
func SaveConfigValue(key, value string) error {
	envPath, err := GetEnvFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(envPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("falha ao criar pasta de configuração '%s': %w", dir, err)
	}

	// Lê as linhas existentes
	envMap := make(map[string]string)
	if data, err := os.ReadFile(envPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				envMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	// Atualiza a chave desejada
	envMap[strings.ToUpper(key)] = value

	// Reescreve o arquivo .env
	var builder strings.Builder
	builder.WriteString("# 🍬 Caramel CLI - Configurações Locais do Usuário\n")
	for k, v := range envMap {
		builder.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}

	if err := os.WriteFile(envPath, []byte(builder.String()), 0600); err != nil {
		return fmt.Errorf("falha ao salvar arquivo de configuração '%s': %w", envPath, err)
	}

	return nil
}

// loadEnvFileToMap lê um arquivo .env e preenche a struct Config
func loadEnvFileToMap(filePath string, cfg *Config) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.ToUpper(strings.TrimSpace(parts[0]))
			v := strings.TrimSpace(parts[1])

			switch k {
			case "OPENROUTER_API_KEY":
				cfg.OpenRouterAPIKey = v
			case "MODEL_IMAGE":
				cfg.ModelImage = v
			case "MODEL_TEXT":
				cfg.ModelText = v
			case "MODEL_TRIAGE":
				cfg.ModelTriage = v
			}
		}
	}
}
