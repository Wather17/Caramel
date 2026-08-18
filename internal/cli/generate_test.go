package cli

import (
	"testing"
)

func TestImageGenerateCmd_NoInputProvided(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "test-key")

	// Reset flags
	genItemsStr = ""
	genFilePath = ""
	genTheme = ""

	err := imageGenerateCmd.RunE(imageGenerateCmd, []string{})
	if err == nil {
		t.Errorf("esperava erro ao chamar generate sem itens nem tema")
	}
}

func TestImageGenerateCmd_NoAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "")

	err := imageGenerateCmd.RunE(imageGenerateCmd, []string{"maçã", "banana"})
	if err == nil {
		t.Errorf("esperava erro por ausência de chave de API")
	}
}
