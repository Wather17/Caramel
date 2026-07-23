package prompts_test

import (
	"strings"
	"testing"

	"caramel/internal/prompts"
)

func TestGetColorizationPrompt(t *testing.T) {
	prompt := prompts.GetColorizationPrompt()
	if prompt == "" {
		t.Error("Esperado prompt de coloração não vazio")
	}
	if !strings.Contains(prompt, "educational") && !strings.Contains(prompt, "Colorize") {
		t.Errorf("Prompt não contém as palavras-chave esperadas: %s", prompt)
	}
}

func TestGetRoutinePrompt(t *testing.T) {
	prompt := prompts.GetRoutinePrompt()
	if prompt == "" {
		t.Error("Esperado prompt de rotinas não vazio")
	}
	if !strings.Contains(prompt, "pedagogical") && !strings.Contains(prompt, "BNCC") {
		t.Errorf("Prompt de rotinas não contém as palavras-chave esperadas: %s", prompt)
	}
}

