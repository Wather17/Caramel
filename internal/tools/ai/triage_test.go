package ai_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/tools/ai"
)

// setupTriageMock redireciona a URL da API para um servidor de teste e retorna o servidor.
// O handler recebe o payload da requisição já decodificado para inspeção.
func setupTriageMock(t *testing.T, handler func(w http.ResponseWriter, reqBody map[string]interface{})) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("esperado método POST, obtido %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("header Authorization ausente ou inválido: %q", got)
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("falha ao decodificar corpo da requisição: %v", err)
		}
		handler(w, reqBody)
	}))

	original := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = original })

	return server
}

// writeTinyTestImage grava um PNG mínimo válido em disco
func writeTinyTestImage(t *testing.T) string {
	t.Helper()

	// PNG 1x1 preto (base64 fixo e válido)
	const tinyBlackPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	data, err := base64.StdEncoding.DecodeString(tinyBlackPNG)
	if err != nil {
		t.Fatalf("falha ao decodificar base64 de teste: %v", err)
	}

	path := filepath.Join(t.TempDir(), "tiny.png")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("falha ao gravar PNG de teste: %v", err)
	}
	return path
}

func TestTriageImage_AprovaColoracao(t *testing.T) {
	setupTriageMock(t, func(w http.ResponseWriter, reqBody map[string]interface{}) {
		// Valida que o modelo padrão de triagem foi usado
		if model := reqBody["model"]; model != ai.DefaultTriageModel {
			t.Errorf("modelo inesperado na requisição: %v", model)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "{\"should_colorize\": true, \"reason\": \"ilustração em preto e branco de um gato\"}"
				}
			}]
		}`)
	})

	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}

	imgPath := writeTinyTestImage(t)
	res, err := client.TriageImage(imgPath, "prompt de teste", "")
	if err != nil {
		t.Fatalf("TriageImage falhou: %v", err)
	}
	if !res.ShouldColorize {
		t.Error("esperado ShouldColorize=true")
	}
	if res.Reason == "" {
		t.Error("esperado Reason preenchido")
	}
}

func TestTriageImage_RejeitaColoracao(t *testing.T) {
	setupTriageMock(t, func(w http.ResponseWriter, reqBody map[string]interface{}) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "{\"should_colorize\": false, \"reason\": \"imagem contém apenas texto\"}"
				}
			}]
		}`)
	})

	client, _ := ai.NewClient("sk-test")
	res, err := client.TriageImage(writeTinyTestImage(t), "prompt", "")
	if err != nil {
		t.Fatalf("TriageImage falhou: %v", err)
	}
	if res.ShouldColorize {
		t.Error("esperado ShouldColorize=false")
	}
}

func TestTriageImage_RespostaComMarkdown(t *testing.T) {
	setupTriageMock(t, func(w http.ResponseWriter, reqBody map[string]interface{}) {
		w.Header().Set("Content-Type", "application/json")
		content := "Aqui está a análise:\n```json\n{\"should_colorize\": false, \"reason\": \"apenas uma tabela\"}\n```"
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"role": "assistant", "content": content}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	client, _ := ai.NewClient("sk-test")
	res, err := client.TriageImage(writeTinyTestImage(t), "prompt", "")
	if err != nil {
		t.Fatalf("TriageImage falhou com resposta em markdown: %v", err)
	}
	if res.ShouldColorize {
		t.Error("esperado ShouldColorize=false para resposta com fences de markdown")
	}
}

func TestTriageImage_ErroHTTP(t *testing.T) {
	setupTriageMock(t, func(w http.ResponseWriter, reqBody map[string]interface{}) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error": {"message": "rate limit exceeded", "code": 429}}`)
	})

	client, _ := ai.NewClient("sk-test")
	_, err := client.TriageImage(writeTinyTestImage(t), "prompt", "")
	if err == nil {
		t.Error("esperado erro quando a API retorna status 429")
	}
}

func TestTriageImage_RespostaSemJSON(t *testing.T) {
	setupTriageMock(t, func(w http.ResponseWriter, reqBody map[string]interface{}) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Não consigo analisar esta imagem."
				}
			}]
		}`)
	})

	client, _ := ai.NewClient("sk-test")
	_, err := client.TriageImage(writeTinyTestImage(t), "prompt", "")
	if err == nil {
		t.Error("esperado erro quando a resposta não contém JSON (caller deve tratar como fail-open)")
	}
}

func TestTriageImage_ImagemInexistente(t *testing.T) {
	client, _ := ai.NewClient("sk-test")
	_, err := client.TriageImage(filepath.Join(t.TempDir(), "inexistente.png"), "prompt", "")
	if err == nil {
		t.Error("esperado erro ao enviar imagem inexistente para triagem")
	}
}
