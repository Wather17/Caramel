package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Papéis de modelo usados pelo Caramel para filtrar o catálogo da OpenRouter
const (
	RoleImage  = "image"
	RoleText   = "text"
	RoleTriage = "triage"
)

// OpenRouterModelsURL é o endpoint público de listagem de modelos do OpenRouter.
// Declarado como var para permitir a substituição por servidor de teste (httptest).
var OpenRouterModelsURL = "https://openrouter.ai/api/v1/models"

// Model representa um modelo disponível na OpenRouter, com os campos
// relevantes para o Caramel (id, nome, preço, modalidades e contexto).
type Model struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ContextLength int      `json:"context_length"`
	Architecture  Arch     `json:"architecture"`
	Pricing       Pricing  `json:"pricing"`
	PromptPrice   float64  `json:"-"`
}

// Arch descreve as modalidades de entrada/saída suportadas pelo modelo
type Arch struct {
	Modality         string   `json:"modality"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

// Pricing traz os preços por token/imagem (em USD), como strings na API
type Pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Image      string `json:"image"`
	Request    string `json:"request"`
}

// modelsResponse é o envelope de resposta do GET /api/v1/models
type modelsResponse struct {
	Data       []Model `json:"data"`
	TotalCount int     `json:"total_count"`
	Links      struct {
		Next string `json:"next"`
	} `json:"links"`
}

// ListModels obtém todos os modelos disponíveis na OpenRouter.
// O endpoint é público (não exige chave de API) e paginado; a função
// percorre as páginas seguindo o campo 'links.next'.
func ListModels() ([]Model, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	var all []Model

	next := fmt.Sprintf("%s?limit=500", OpenRouterModelsURL)
	for next != "" {
		resp, err := client.Get(next)
		if err != nil {
			return nil, fmt.Errorf("falha ao consultar modelos da OpenRouter: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("falha ao listar modelos da OpenRouter: status %d", resp.StatusCode)
		}

		var r modelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("falha ao interpretar resposta de modelos: %w", err)
		}
		resp.Body.Close()

		all = append(all, r.Data...)
		next = resolveModelsNextURL(r.Links.Next)
	}

	for i := range all {
		all[i].PromptPrice = parsePrice(all[i].Pricing.Prompt)
	}
	return all, nil
}

// resolveModelsNextURL completa URLs relativas de paginação (ex: '/api/v1/models?offset=...')
func resolveModelsNextURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.IsAbs() {
		return u.String()
	}
	base, err := url.Parse(OpenRouterModelsURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}

// parsePrice converte uma string de preço (ex: "0.0000005") em float64
func parsePrice(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// inModalities retorna as modalidades de entrada (com fallback para o campo modality)
func (m Model) inModalities() []string {
	if len(m.Architecture.InputModalities) > 0 {
		return m.Architecture.InputModalities
	}
	if idx := strings.Index(m.Architecture.Modality, "->"); idx >= 0 {
		return strings.Split(m.Architecture.Modality[:idx], "+")
	}
	return nil
}

// outModalities retorna as modalidades de saída (com fallback para o campo modality)
func (m Model) outModalities() []string {
	if len(m.Architecture.OutputModalities) > 0 {
		return m.Architecture.OutputModalities
	}
	if idx := strings.Index(m.Architecture.Modality, "->"); idx >= 0 {
		return strings.Split(m.Architecture.Modality[idx+2:], "+")
	}
	return nil
}

// IsImageModel indica se o modelo gera imagens (ex: google/gemini-3.1-flash-image-preview)
func (m Model) IsImageModel() bool {
	return containsString(m.outModalities(), "image")
}

// IsVisionModel indica se o modelo lê imagens e responde em texto (ideal para a triagem)
func (m Model) IsVisionModel() bool {
	return containsString(m.inModalities(), "image") && !containsString(m.outModalities(), "image")
}

// IsTextModel indica se o modelo é de texto puro (sem entrada de imagem)
func (m Model) IsTextModel() bool {
	return !containsString(m.inModalities(), "image") && containsString(m.outModalities(), "text")
}

func containsString(list []string, target string) bool {
	for _, s := range list {
		if strings.TrimSpace(s) == target {
			return true
		}
	}
	return false
}

// FilterModelsByRole retorna apenas os modelos que atendem ao papel solicitado
// (image, text ou triage). Mantém a ordem original da lista.
func FilterModelsByRole(models []Model, role string) []Model {
	var out []Model
	for _, m := range models {
		var ok bool
		switch role {
		case RoleImage:
			ok = m.IsImageModel()
		case RoleTriage:
			ok = m.IsVisionModel()
		case RoleText:
			ok = m.IsTextModel()
		default:
			ok = m.IsTextModel()
		}
		if ok {
			out = append(out, m)
		}
	}
	return out
}

// SearchModels filtra os modelos cujo id ou nome contém o termo (case-insensitive)
func SearchModels(models []Model, term string) []Model {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return models
	}
	var out []Model
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.ID), term) ||
			strings.Contains(strings.ToLower(m.Name), term) {
			out = append(out, m)
		}
	}
	return out
}

// PricedModels remove roteadores automáticos e modelos sem preço publicado
// (a API retorna preço -1 para o openrouter/auto), mantendo apenas modelos reais.
func PricedModels(models []Model) []Model {
	var out []Model
	for _, m := range models {
		if m.PromptPrice >= 0 {
			out = append(out, m)
		}
	}
	return out
}

// SortModelsByPrice ordena os modelos do mais barato para o mais caro (preço/M prompt)
func SortModelsByPrice(models []Model) {
	sort.SliceStable(models, func(i, j int) bool {
		return models[i].PromptPrice < models[j].PromptPrice
	})
}

// SortModelsByName ordena os modelos por id (ordem alfabética)
func SortModelsByName(models []Model) {
	sort.SliceStable(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
}