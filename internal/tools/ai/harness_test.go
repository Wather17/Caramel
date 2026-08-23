package ai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"caramel/internal/tools/ai"
)

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		index    int
		name     string
		expected string
	}{
		{1, "Maçã", "01_maçã"},
		{2, "Banana Prata", "02_banana_prata"},
		{10, "Cachorro & Gato", "10_cachorro__gato"},
		{3, "   Espaços   ", "03_espaços"},
		{5, "!!!", "05_item"},
	}

	for _, tt := range tests {
		got := ai.SanitizeSlug(tt.index, tt.name)
		if got != tt.expected {
			t.Errorf("SanitizeSlug(%d, %q) = %q; esperado %q", tt.index, tt.name, got, tt.expected)
		}
	}
}

func TestCalculateConcurrencyDecision(t *testing.T) {
	// 1. Direct Burst (N <= 3)
	workers, delay := ai.CalculateConcurrencyDecision(2, 0)
	if workers != 2 || delay != 0 {
		t.Errorf("esperado (2, 0ms) para N=2, obtido (%d, %v)", workers, delay)
	}

	// 2. Managed Pool (3 < N <= 10)
	workers, delay = ai.CalculateConcurrencyDecision(8, 0)
	if workers != 4 || delay != 150*time.Millisecond {
		t.Errorf("esperado (4, 150ms) para N=8, obtido (%d, %v)", workers, delay)
	}

	// 3. Adaptive Throttle (N > 10)
	workers, delay = ai.CalculateConcurrencyDecision(25, 0)
	if workers != 5 || delay != 300*time.Millisecond {
		t.Errorf("esperado (5, 300ms) para N=25, obtido (%d, %v)", workers, delay)
	}

	// 4. User Override
	workers, delay = ai.CalculateConcurrencyDecision(25, 10)
	if workers != 10 {
		t.Errorf("esperado 10 workers configurados manualmente, obtido %d", workers)
	}
}

// setupAnalyzeMock redireciona a API para um servidor fake que responde com
// conteúdo de texto (usado por SynthesizePrompts via AnalyzeRoutine)
func setupAnalyzeMock(t *testing.T, content string, capture func(reqBody map[string]interface{})) {
	t.Helper()

	payload := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%s}}]}`, mustJSONString(t, content))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			var reqBody map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				t.Errorf("falha ao decodificar requisição: %v", err)
			}
			capture(reqBody)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, payload)
	}))

	old := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = srv.URL
	t.Cleanup(func() {
		ai.OpenRouterAPIURL = old
		srv.Close()
	})
}

func mustJSONString(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("falha ao serializar conteúdo: %v", err)
	}
	return string(b)
}

func TestSynthesizePromptsComItens(t *testing.T) {
	listaJSON := "```json\n[" +
		`{"name":"Leite","prompt":"copo de leite clipart"},` +
		`{"name":"Pão Francês","slug":"pao","prompt":"pão francês clipart"}` +
		"]\n```"
	setupAnalyzeMock(t, listaJSON, nil)

	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatalf("NewClient falhou: %v", err)
	}

	cfg := ai.HarnessConfig{Items: []string{"Leite", "Pão Francês"}, Style: "clipart"}
	items, err := ai.SynthesizePrompts(cfg, client)
	if err != nil {
		t.Fatalf("SynthesizePrompts falhou: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("esperado 2 itens, obtido %d", len(items))
	}
	if items[0].Index != 1 || items[1].Index != 2 {
		t.Errorf("índices deveriam ser normalizados para 1 e 2, obtido %+v", items)
	}
	for _, it := range items {
		if it.Status != "pending" {
			t.Errorf("itens sintetizados deveriam iniciar como 'pending', obtido %q", it.Status)
		}
		if it.Slug == "" {
			t.Errorf("slug deveria ser gerado automaticamente para %q", it.Name)
		}
	}
	if items[0].Slug != "01_leite" {
		t.Errorf("slug esperado '01_leite', obtido %q", items[0].Slug)
	}
	if items[1].Slug != "pao" {
		t.Errorf("slug informado pela IA deveria ser preservado, obtido %q", items[1].Slug)
	}
}

func TestSynthesizePromptsPorTema(t *testing.T) {
	var capturado map[string]interface{}
	setupAnalyzeMock(t, `[{"name":"Fazenda","prompt":"cenário de fazenda"}]`, func(req map[string]interface{}) {
		capturado = req
	})

	client, _ := ai.NewClient("sk-test")
	cfg := ai.HarnessConfig{Theme: "Fazenda", Count: 5, TextModel: "m/texto"}
	items, err := ai.SynthesizePrompts(cfg, client)
	if err != nil {
		t.Fatalf("SynthesizePrompts falhou: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Fazenda" {
		t.Errorf("item inesperado: %+v", items)
	}

	msgs, _ := capturado["messages"].([]interface{})
	if len(msgs) == 0 {
		t.Fatal("requisição deveria ter mensagens")
	}
	parts, _ := msgs[0].(map[string]interface{})["content"].([]interface{})
	texto, _ := parts[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(texto, "THEME: Fazenda") || !strings.Contains(texto, "5 most iconic") {
		t.Errorf("prompt deveria conter tema e contagem, obtido: %s", texto)
	}
	if capturado["model"] != "m/texto" {
		t.Errorf("modelo configurado deveria ser usado, obtido %v", capturado["model"])
	}
}

func TestSynthesizePromptsSemEntrada(t *testing.T) {
	client, _ := ai.NewClient("sk-test")
	cfg := ai.HarnessConfig{} // nem itens nem tema

	if _, err := ai.SynthesizePrompts(cfg, client); err == nil {
		t.Error("sem itens nem tema deveria retornar erro sem chamar a API")
	}
}

func TestExecuteGenerationHarnessSucesso(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="}}]}}]}`)
	}))
	defer srv.Close()

	old := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = srv.URL
	defer func() { ai.OpenRouterAPIURL = old }()

	outDir := t.TempDir()
	items := []ai.GenerationItem{
		{Index: 1, Name: "Leite", Slug: "01_leite", Prompt: "copo de leite"},
		{Index: 2, Name: "Pão", Slug: "02_pao", Prompt: "pão francês"},
	}

	var doneEvents int
	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatalf("NewClient falhou: %v", err)
	}
	cfg := ai.HarnessConfig{OutputDir: outDir, MaxWorkers: 2, ImageModel: "m/imagem"}
	results, err := ai.ExecuteGenerationHarness(items, cfg, client, func(ev ai.HarnessProgressEvent) {
		if ev.CurrentStep == "done" && ev.Success == 2 && ev.Failed == 0 {
			doneEvents++
		}
	})
	if err != nil {
		t.Fatalf("ExecuteGenerationHarness falhou: %v", err)
	}
	if doneEvents != 1 {
		t.Errorf("esperado exatamente 1 evento final com sucesso=2, obtido %d", doneEvents)
	}

	for i, res := range results {
		if res.Status != "done" || res.ImagePath == "" {
			t.Errorf("item %d deveria estar 'done' com caminho preenchido, obtido %+v", i, res)
			continue
		}
		if _, err := os.Stat(res.ImagePath); err != nil {
			t.Errorf("arquivo do item %d não foi salvo: %v", i, err)
		}
	}
}

func TestExecuteGenerationHarnessErroDaAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"sem créditos"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	old := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = srv.URL
	defer func() { ai.OpenRouterAPIURL = old }()

	outDir := t.TempDir()
	items := []ai.GenerationItem{{Index: 1, Name: "X", Slug: "01_x", Prompt: "teste"}}

	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatalf("NewClient falhou: %v", err)
	}
	results, err := ai.ExecuteGenerationHarness(items, ai.HarnessConfig{OutputDir: outDir}, client, nil)
	if err != nil {
		t.Fatalf("erro da API não deveria abortar o harness: %v", err)
	}
	if results[0].Status != "error" || results[0].Error == "" {
		t.Errorf("item deveria terminar em 'error' com mensagem, obtido %+v", results[0])
	}
	matches, _ := filepath.Glob(filepath.Join(outDir, "*"))
	if len(matches) != 0 {
		t.Errorf("nenhum arquivo deveria ser salvo em caso de falha, obtido %v", matches)
	}
}
