package ai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"caramel/internal/tools/ai"
)

// mockModel é um modelo mínimo para os testes de catálogo
type mockModel struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ContextLength int      `json:"context_length"`
	Architecture  struct {
		Modality         string   `json:"modality"`
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"architecture"`
	Pricing struct {
		Prompt string `json:"prompt"`
	} `json:"pricing"`
}

func mockJSON(t *testing.T, models []mockModel, next string) string {
	t.Helper()
	payload := map[string]any{
		"data":        models,
		"total_count": len(models),
		"links":       map[string]any{"next": next},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("falha ao gerar fixture JSON: %v", err)
	}
	return string(b)
}

func modelWith(modality string, in, out []string, id, price string) mockModel {
	m := mockModel{ID: id, Name: "Modelo " + id}
	m.Architecture.Modality = modality
	m.Architecture.InputModalities = in
	m.Architecture.OutputModalities = out
	m.Pricing.Prompt = price
	return m
}

func TestListModelsPaginated(t *testing.T) {
	page1 := mockJSON(t, []mockModel{
		modelWith("text->text", []string{"text"}, []string{"text"}, "deepseek/deepseek-v4-flash", "0.0000003"),
	}, "/api/v1/models?offset=1&limit=500")
	page2 := mockJSON(t, []mockModel{
		modelWith("text+image->image", []string{"text", "image"}, []string{"image", "text"}, "google/gemini-3.1-flash-image-preview", "0.0000005"),
	}, "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "1" {
			fmt.Fprint(w, page2)
			return
		}
		fmt.Fprint(w, page1)
	}))
	defer srv.Close()

	ai.OpenRouterModelsURL = srv.URL

	models, err := ai.ListModels()
	if err != nil {
		t.Fatalf("ListModels falhou: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("Esperado 2 modelos paginados, obtido %d", len(models))
	}
	if models[1].ID != "google/gemini-3.1-flash-image-preview" {
		t.Errorf("Esperado modelo da segunda página como último, obtido %s", models[1].ID)
	}
	if models[0].PromptPrice != 0.0000003 {
		t.Errorf("Esperado PromptPrice 0.0000003, obtido %v", models[0].PromptPrice)
	}
}

func TestListModelsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ai.OpenRouterModelsURL = srv.URL

	if _, err := ai.ListModels(); err == nil {
		t.Error("Esperava erro para resposta HTTP 500")
	}
}

func TestFilterModelsByRole(t *testing.T) {
	models := []ai.Model{
		{ID: "a/imagem", Name: "Imagem", Architecture: ai.Arch{OutputModalities: []string{"image"}}, PromptPrice: 0.5},
		{ID: "a/visao", Name: "Visao", Architecture: ai.Arch{InputModalities: []string{"image"}, OutputModalities: []string{"text"}}, PromptPrice: 0.1},
		{ID: "a/texto", Name: "Texto", Architecture: ai.Arch{InputModalities: []string{"text"}, OutputModalities: []string{"text"}}, PromptPrice: 0.05},
		{ID: "a/multimodal", Name: "Multimodal", Architecture: ai.Arch{InputModalities: []string{"text", "image"}, OutputModalities: []string{"text"}}, PromptPrice: 0.2},
	}

	img := ai.FilterModelsByRole(models, ai.RoleImage)
	if len(img) != 1 || img[0].ID != "a/imagem" {
		t.Errorf("Role image: esperado apenas 'a/imagem', obtido %+v", img)
	}

	vis := ai.FilterModelsByRole(models, ai.RoleTriage)
	if len(vis) != 2 {
		t.Errorf("Role triage: esperado 2 modelos de visão, obtido %d", len(vis))
	}

	txt := ai.FilterModelsByRole(models, ai.RoleText)
	if len(txt) != 1 || txt[0].ID != "a/texto" {
		t.Errorf("Role text: esperado apenas 'a/texto', obtido %+v", txt)
	}
}

func TestSearchModelsAndSortByPrice(t *testing.T) {
	models := []ai.Model{
		{ID: "google/gemini-pro", Name: "Gemini Pro"},
		{ID: "openai/gpt-4", Name: "GPT-4"},
		{ID: "google/gemini-flash", Name: "Gemini Flash"},
	}

	found := ai.SearchModels(models, "gemini")
	if len(found) != 2 {
		t.Errorf("Busca 'gemini': esperado 2, obtido %d", len(found))
	}

	found = ai.SearchModels(models, "")
	if len(found) != 3 {
		t.Errorf("Busca vazia deveria retornar todos, obtido %d", len(found))
	}
}

func TestPricedModels(t *testing.T) {
	models := []ai.Model{
		{ID: "openrouter/auto", PromptPrice: -1},
		{ID: "gratis/model", PromptPrice: 0},
		{ID: "pago/model", PromptPrice: 0.5},
	}

	got := ai.PricedModels(models)
	if len(got) != 2 || got[0].ID != "gratis/model" || got[1].ID != "pago/model" {
		t.Errorf("esperado apenas modelos com preço >= 0 na ordem original, obtido %+v", got)
	}
	if len(ai.PricedModels(nil)) != 0 {
		t.Error("lista nula deveria retornar lista vazia")
	}
}

func TestSortModelsByPriceAndName(t *testing.T) {
	baratos := []ai.Model{
		{ID: "c/caro", PromptPrice: 3.0},
		{ID: "a/barato", PromptPrice: 0.1},
		{ID: "b/medio", PromptPrice: 1.5},
	}
	ai.SortModelsByPrice(baratos)
	for i, id := range []string{"a/barato", "b/medio", "c/caro"} {
		if baratos[i].ID != id {
			t.Fatalf("SortModelsByPrice: esperado %s na posição %d, obtido %s", id, i, baratos[i].ID)
		}
	}

	nomes := []ai.Model{{ID: "z/model"}, {ID: "a/model"}, {ID: "m/model"}}
	ai.SortModelsByName(nomes)
	for i, id := range []string{"a/model", "m/model", "z/model"} {
		if nomes[i].ID != id {
			t.Fatalf("SortModelsByName: esperado %s na posição %d, obtido %s", id, i, nomes[i].ID)
		}
	}
}

func TestModalitiesFallbackViaPredicados(t *testing.T) {
	tests := []struct {
		name          string
		mod           ai.Model
		wantImage     bool
		wantVision    bool
		wantTextModel bool
	}{
		{
			name:       "fallback para campo modality texto->texto",
			mod:        ai.Model{Architecture: ai.Arch{Modality: "text->text"}},
			wantTextModel: true,
		},
		{
			name:      "fallback multimodal text+image->image",
			mod:       ai.Model{Architecture: ai.Arch{Modality: "text+image->image"}},
			wantImage: true,
		},
		{
			name: "listas explicitas tem prioridade sobre modality",
			mod: ai.Model{Architecture: ai.Arch{
				Modality:         "text->text",
				InputModalities:  []string{"image"},
				OutputModalities: []string{"text"},
			}},
			wantVision: true,
		},
		{
			name: "sem modalidades nao é nada",
			mod:  ai.Model{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mod.IsImageModel(); got != tt.wantImage {
				t.Errorf("IsImageModel() = %v, esperado %v", got, tt.wantImage)
			}
			if got := tt.mod.IsVisionModel(); got != tt.wantVision {
				t.Errorf("IsVisionModel() = %v, esperado %v", got, tt.wantVision)
			}
			if got := tt.mod.IsTextModel(); got != tt.wantTextModel {
				t.Errorf("IsTextModel() = %v, esperado %v", got, tt.wantTextModel)
			}
		})
	}
}