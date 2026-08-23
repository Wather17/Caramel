package prompts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/prompts"
)

// useCustomConfigDir aponta a pasta de configuração do Caramel para um diretório
// temporário, permitindo testar os overrides de prompt do usuário.
func useCustomConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	promptsDir := filepath.Join(dir, "caramel", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("falha ao criar pasta de prompts customizados: %v", err)
	}
	return promptsDir
}

func TestGetColorizationPrompt(t *testing.T) {
	prompt := prompts.GetColorizationPrompt()
	if prompt == "" {
		t.Error("Esperado prompt de coloração não vazio")
	}
	if !strings.Contains(prompt, "educational") && !strings.Contains(prompt, "Colorize") {
		t.Errorf("Prompt não contém as palavras-chave esperadas: %s", prompt)
	}
}

func TestGetPromptSynthesizerPrompt(t *testing.T) {
	prompt := prompts.GetPromptSynthesizerPrompt("3d-cute")
	if prompt == "" {
		t.Error("Esperado prompt sintetizador não vazio")
	}
	if !strings.Contains(prompt, "3d-cute") {
		t.Errorf("Esperado estilo '3d-cute' injetado no prompt, obtido: %s", prompt)
	}
}

func TestGetTriagePrompt(t *testing.T) {
	prompt := prompts.GetTriagePrompt()
	if prompt == "" {
		t.Error("Esperado prompt de triagem não vazio")
	}
	if !strings.Contains(prompt, "should_colorize") {
		t.Errorf("Prompt de triagem deve exigir resposta JSON com a chave 'should_colorize', obtido: %s", prompt)
	}
}

func TestGetRoutinePrompt(t *testing.T) {
	prompt := prompts.GetRoutinePrompt()
	if prompt == "" {
		t.Fatal("Esperado prompt de rotinas não vazio")
	}
	if !strings.Contains(prompt, "BNCC") {
		t.Errorf("Prompt de rotinas pedagógicas deveria referenciar a BNCC, obtido: %s", prompt)
	}
}

func TestCustomPromptOverrides(t *testing.T) {
	promptsDir := useCustomConfigDir(t)

	custom := map[string]string{
		"colorization.txt":       "CUSTOM colorization prompt",
		"triage.txt":             "CUSTOM triage prompt should_colorize",
		"routine.txt":            "CUSTOM routine prompt BNCC",
		"prompt_synthesizer.txt": "CUSTOM synthesizer {{.Style}}",
	}
	for name, content := range custom {
		if err := os.WriteFile(filepath.Join(promptsDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("falha ao escrever override %s: %v", name, err)
		}
	}

	if got := prompts.GetColorizationPrompt(); got != custom["colorization.txt"] {
		t.Errorf("override de colorização não aplicado, obtido: %q", got)
	}
	if got := prompts.GetTriagePrompt(); got != custom["triage.txt"] {
		t.Errorf("override de triagem não aplicado, obtido: %q", got)
	}
	if got := prompts.GetRoutinePrompt(); got != custom["routine.txt"] {
		t.Errorf("override de rotina não aplicado, obtido: %q", got)
	}
	if got := prompts.GetPromptSynthesizerPrompt("3d-cute"); got != "CUSTOM synthesizer 3d-cute" {
		t.Errorf("override do sintetizador deveria substituir {{.Style}}, obtido: %q", got)
	}
}

func TestEmptyOrWhitespaceOverrideFallsBackToEmbedded(t *testing.T) {
	promptsDir := useCustomConfigDir(t)

	for _, name := range []string{"colorization.txt", "triage.txt"} {
		if err := os.WriteFile(filepath.Join(promptsDir, name), []byte("   \n\t "), 0644); err != nil {
			t.Fatalf("falha ao escrever override vazio: %v", err)
		}
	}

	if got := prompts.GetColorizationPrompt(); strings.Contains(got, "\t") || got == "" {
		t.Errorf("override em branco deveria cair no prompt embarcado, obtido: %q", got)
	}
	if !strings.Contains(prompts.GetTriagePrompt(), "should_colorize") {
		t.Error("fallback embarcado da triagem deveria ser usado quando o override é branco")
	}
}


