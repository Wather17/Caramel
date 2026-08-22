package cli

import (
	"strings"
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

func TestValidateAspect(t *testing.T) {
	// Default 1:1 deve ser aceito, assim como todos os valores suportados
	for _, valid := range validAspects {
		if err := validateAspect(valid); err != nil {
			t.Errorf("proporção '%s' deveria ser válida: %v", valid, err)
		}
	}
	if err := validateAspect("1:1"); err != nil {
		t.Errorf("default '1:1' deveria ser válido: %v", err)
	}

	// Valores inválidos devem falhar e a mensagem deve listar os aceitos
	invalid := []string{"", "5:5", "1x1", "quadrado", "16/9", "foo"}
	for _, v := range invalid {
		err := validateAspect(v)
		if err == nil {
			t.Errorf("proporção inválida '%s' deveria retornar erro", v)
			continue
		}
		if !strings.Contains(err.Error(), "1:1") {
			t.Errorf("mensagem de erro deveria listar valores aceitos: %v", err)
		}
	}
}

func TestImageGenerateCmd_AspectInvalido(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "test-key")

	genAspect = "5:5"
	genItemsStr = "maçã"
	genTheme = ""
	genFilePath = ""

	err := imageGenerateCmd.RunE(imageGenerateCmd, []string{})
	if err == nil {
		t.Fatal("esperava erro ao usar proporção inválida")
	}
	if !strings.Contains(err.Error(), "Valores aceitos") {
		t.Errorf("mensagem de erro deveria listar valores aceitos: %v", err)
	}
}
