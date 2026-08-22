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


