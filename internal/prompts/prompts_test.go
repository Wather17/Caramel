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
