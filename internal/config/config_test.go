package config_test

import (
	"os"
	"testing"

	"caramel/internal/config"
)

func TestGetConfigDir(t *testing.T) {
	dir, err := config.GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir falhou: %v", err)
	}
	if dir == "" {
		t.Error("Esperado diretório de configuração não vazio")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Cria diretório temporário simulando XDG_CONFIG_HOME
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	testKey := "OPENROUTER_API_KEY"
	testValue := "sk-or-v1-teste123456789"

	err := config.SaveConfigValue(testKey, testValue)
	if err != nil {
		t.Fatalf("SaveConfigValue falhou: %v", err)
	}

	envPath, err := config.GetEnvFilePath()
	if err != nil {
		t.Fatalf("GetEnvFilePath falhou: %v", err)
	}

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Fatalf("Arquivo .env de configuração não foi encontrado em: %s", envPath)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig falhou: %v", err)
	}

	if cfg.OpenRouterAPIKey != testValue {
		t.Errorf("Esperado valor %s, obtido %s", testValue, cfg.OpenRouterAPIKey)
	}
}
