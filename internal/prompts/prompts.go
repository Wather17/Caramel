package prompts

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
)

//go:embed templates/*
var templateFS embed.FS

// GetColorizationPrompt retorna o prompt de coloração.
// Se o usuário tiver um prompt customizado em ~/.config/caramel/prompts/colorization.txt, ele será usado.
// Caso contrário, retorna o prompt embarcado por padrão no binário.
func GetColorizationPrompt() string {
	// 1. Tenta carregar override do usuário na pasta de configuração
	configDir, err := config.GetConfigDir()
	if err == nil {
		customPromptPath := filepath.Join(configDir, "prompts", "colorization.txt")
		if data, err := os.ReadFile(customPromptPath); err == nil {
			content := strings.TrimSpace(string(data))
			if content != "" {
				return content
			}
		}
	}

	// 2. Fallback para o prompt embarcado nativamente no binário Go
	data, err := templateFS.ReadFile("templates/colorization.txt")
	if err != nil {
		return "Colorize this black and white educational illustration with vibrant, friendly, high-quality colors."
	}

	return strings.TrimSpace(string(data))
}

// GetTriagePrompt retorna o prompt de triagem (gatekeeper) que decide se uma imagem
// deve ser enviada para a etapa cara de coloração.
// Se o usuário tiver um prompt customizado em ~/.config/caramel/prompts/triage.txt, ele será usado.
// Caso contrário, retorna o prompt embarcado por padrão no binário.
func GetTriagePrompt() string {
	configDir, err := config.GetConfigDir()
	if err == nil {
		customPromptPath := filepath.Join(configDir, "prompts", "triage.txt")
		if data, err := os.ReadFile(customPromptPath); err == nil {
			content := strings.TrimSpace(string(data))
			if content != "" {
				return content
			}
		}
	}

	data, err := templateFS.ReadFile("templates/triage.txt")
	if err != nil {
		return "Decide whether this black-and-white image is a colorable illustration. Respond only with JSON: {\"should_colorize\": true|false, \"reason\": \"...\"}"
	}

	return strings.TrimSpace(string(data))
}

// GetRoutinePrompt retorna o prompt para o assistente de rotinas pedagógicas.
// Se o usuário tiver um prompt customizado em ~/.config/caramel/prompts/routine.txt, ele será usado.
// Caso contrário, retorna o prompt embarcado por padrão no binário.
func GetRoutinePrompt() string {
	configDir, err := config.GetConfigDir()
	if err == nil {
		customPromptPath := filepath.Join(configDir, "prompts", "routine.txt")
		if data, err := os.ReadFile(customPromptPath); err == nil {
			content := strings.TrimSpace(string(data))
			if content != "" {
				return content
			}
		}
	}

	data, err := templateFS.ReadFile("templates/routine.txt")
	if err != nil {
		return "Analyze the weekly pedagogical routine and summarize the experiences according to BNCC."
	}

	return strings.TrimSpace(string(data))
}

// GetPromptSynthesizerPrompt retorna o template do prompt para sintetizar listas em prompts de imagens
func GetPromptSynthesizerPrompt(style string) string {
	if style == "" {
		style = "clipart"
	}

	configDir, err := config.GetConfigDir()
	if err == nil {
		customPromptPath := filepath.Join(configDir, "prompts", "prompt_synthesizer.txt")
		if data, err := os.ReadFile(customPromptPath); err == nil {
			content := strings.TrimSpace(string(data))
			if content != "" {
				return strings.ReplaceAll(content, "{{.Style}}", style)
			}
		}
	}

	data, err := templateFS.ReadFile("templates/prompt_synthesizer.txt")
	if err != nil {
		return "Generate text-to-image prompts in JSON format for the following items in " + style + " style."
	}

	templateStr := strings.TrimSpace(string(data))
	return strings.ReplaceAll(templateStr, "{{.Style}}", style)
}


