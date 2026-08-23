package config_test

import (
	"os"
	"path/filepath"
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

func TestSaveAndLoadModels(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	values := map[string]string{
		"MODEL_IMAGE":  "google/gemini-3.1-flash-image-preview",
		"MODEL_TEXT":   "deepseek/deepseek-v4-flash",
		"MODEL_TRIAGE": "qwen/qwen3.7-flash",
	}
	for k, v := range values {
		if err := config.SaveConfigValue(k, v); err != nil {
			t.Fatalf("SaveConfigValue(%s) falhou: %v", k, err)
		}
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig falhou: %v", err)
	}

	if cfg.ModelImage != values["MODEL_IMAGE"] {
		t.Errorf("ModelImage: esperado %q, obtido %q", values["MODEL_IMAGE"], cfg.ModelImage)
	}
	if cfg.ModelText != values["MODEL_TEXT"] {
		t.Errorf("ModelText: esperado %q, obtido %q", values["MODEL_TEXT"], cfg.ModelText)
	}
	if cfg.ModelTriage != values["MODEL_TRIAGE"] {
		t.Errorf("ModelTriage: esperado %q, obtido %q", values["MODEL_TRIAGE"], cfg.ModelTriage)
	}
}

func TestEnvModelPriority(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	if err := config.SaveConfigValue("MODEL_IMAGE", "google/do-arquivo"); err != nil {
		t.Fatalf("SaveConfigValue falhou: %v", err)
	}

	// Variável de ambiente do SO tem prioridade máxima
	t.Setenv("MODEL_IMAGE", "google/do-sistema")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig falhou: %v", err)
	}
	if cfg.ModelImage != "google/do-sistema" {
		t.Errorf("Variável do SO deveria ter prioridade: esperado %q, obtido %q", "google/do-sistema", cfg.ModelImage)
	}
}

func TestGetConfigDirComXDG(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	got, err := config.GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir falhou: %v", err)
	}
	want := filepath.Join(tempDir, "caramel")
	if got != want {
		t.Errorf("esperado %q, obtido %q", want, got)
	}
}

func TestGetEnvFilePathComXDG(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	got, err := config.GetEnvFilePath()
	if err != nil {
		t.Fatalf("GetEnvFilePath falhou: %v", err)
	}
	want := filepath.Join(tempDir, "caramel", ".env")
	if got != want {
		t.Errorf("esperado %q, obtido %q", want, got)
	}
}
