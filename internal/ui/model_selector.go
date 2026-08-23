package ui

import (
	"fmt"

	"caramel/internal/tools/ai"

	"github.com/charmbracelet/huh"
)

// ModelSelection agrupa os modelos escolhidos pelo usuário na TUI
type ModelSelection struct {
	ImageModel  string
	TextModel   string
	TriageModel string
}

// ModelDefaults traz os valores atuais (config/.env ou fallback) para pré-seleção
type ModelDefaults struct {
	ImageModel  string
	TextModel   string
	TriageModel string
}

// SelectModelsInteractive exibe uma TUI dividida em três categorias (imagem, texto
// e triagem) com busca incremental — o usuário digita para filtrar os modelos.
// Retorna os ids escolhidos em cada categoria.
func SelectModelsInteractive(image, text, triage []ai.Model, defaults ModelDefaults) (ModelSelection, error) {
	imgValue := defaults.ImageModel
	txtValue := defaults.TextModel
	triValue := defaults.TriageModel

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("🎨 Modelo de Imagem").
				Description("Geração e coloração de imagens. Digite para filtrar (ex: 'gpt image 2').").
				Options(ensureDefaultOption(modelOptions(image), imgValue)...).
				Filtering(true).
				Value(&imgValue),

			huh.NewSelect[string]().
				Title("🧠 Modelo de Texto").
				Description("Síntese de prompts e análise de rotinas. Digite para filtrar.").
				Options(ensureDefaultOption(modelOptions(text), txtValue)...).
				Filtering(true).
				Value(&txtValue),

			huh.NewSelect[string]().
				Title("🔍 Modelo de Triagem (Visão)").
				Description("Análise de economia antes de colorir. Digite para filtrar.").
				Options(ensureDefaultOption(modelOptions(triage), triValue)...).
				Filtering(true).
				Value(&triValue),
		),
	).WithTheme(GetCaramelTheme())

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return ModelSelection{}, fmt.Errorf("seleção de modelos cancelada pelo usuário")
		}
		return ModelSelection{}, fmt.Errorf("falha ao executar seleção de modelos: %w", err)
	}

	return ModelSelection{
		ImageModel:  imgValue,
		TextModel:   txtValue,
		TriageModel: triValue,
	}, nil
}

// modelOptions converte modelos em opções legíveis para o formulário huh
func modelOptions(models []ai.Model) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(models))
	for _, m := range models {
		label := fmt.Sprintf("%s  ·  %s  ·  $%.2f/M", m.ID, m.Name, m.PromptPrice*1e6)
		opts = append(opts, huh.NewOption(label, m.ID))
	}
	return opts
}

// ensureDefaultOption garante que o valor atual esteja nas opções para a
// pré-seleção funcionar, mesmo que ele não esteja na lista exibida.
func ensureDefaultOption(opts []huh.Option[string], current string) []huh.Option[string] {
	if current == "" {
		return opts
	}
	for _, o := range opts {
		if o.Value == current {
			return opts
		}
	}
	return append([]huh.Option[string]{huh.NewOption(fmt.Sprintf("%s  ·  (atual)", current), current)}, opts...)
}