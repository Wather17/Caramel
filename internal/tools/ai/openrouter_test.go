package ai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"caramel/internal/tools/ai"
)

func TestNewClient(t *testing.T) {
	_, err := ai.NewClient("")
	if err == nil {
		t.Error("Esperado erro ao tentar criar cliente sem APIKey")
	}

	client, err := ai.NewClient("sk-or-v1-teste123")
	if err != nil {
		t.Fatalf("NewClient falhou: %v", err)
	}
	if client == nil {
		t.Error("Cliente criado não deve ser nulo")
	}
}

// setupGenerateImageMock redireciona a API para um servidor de teste e captura o corpo
// da requisição para inspeção, devolvendo um PNG via message.images.
func setupGenerateImageMock(t *testing.T, capture func(reqBody map[string]interface{})) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("falha ao decodificar corpo da requisição: %v", err)
		}
		capture(reqBody)

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"choices": [{
				"message": {
					"role": "assistant",
					"images": [{"type": "image_url", "image_url": {"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="}}]
				}
			}]
		}`)
	}))

	original := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() {
		ai.OpenRouterAPIURL = original
		server.Close()
	})
}

func TestGenerateImage_DefaultAspect1x1(t *testing.T) {
	var captured map[string]interface{}
	setupGenerateImageMock(t, func(reqBody map[string]interface{}) {
		captured = reqBody
	})

	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}

	if _, _, err := client.GenerateImage("prompt de teste", "", ""); err != nil {
		t.Fatalf("GenerateImage falhou: %v", err)
	}

	if captured == nil {
		t.Fatal("requisição não foi capturada pelo mock")
	}
	imgCfg, ok := captured["image_config"].(map[string]interface{})
	if !ok {
		t.Fatal("esperado campo image_config na requisição")
	}
	if ratio, _ := imgCfg["aspect_ratio"].(string); ratio != "1:1" {
		t.Errorf("esperado aspect_ratio '1:1' por padrão, obtido '%v'", imgCfg["aspect_ratio"])
	}
}

func TestGenerateImage_AspectCustomizado(t *testing.T) {
	var captured map[string]interface{}
	setupGenerateImageMock(t, func(reqBody map[string]interface{}) {
		captured = reqBody
	})

	client, _ := ai.NewClient("sk-test")

	if _, _, err := client.GenerateImage("prompt", "", "16:9"); err != nil {
		t.Fatalf("GenerateImage falhou: %v", err)
	}

	imgCfg, ok := captured["image_config"].(map[string]interface{})
	if !ok {
		t.Fatal("esperado campo image_config na requisição")
	}
	if ratio, _ := imgCfg["aspect_ratio"].(string); ratio != "16:9" {
		t.Errorf("esperado aspect_ratio '16:9', obtido '%v'", imgCfg["aspect_ratio"])
	}
}

func TestGenerateImage_AspectAuto(t *testing.T) {
	var captured map[string]interface{}
	setupGenerateImageMock(t, func(reqBody map[string]interface{}) {
		captured = reqBody
	})

	client, _ := ai.NewClient("sk-test")

	if _, _, err := client.GenerateImage("prompt", "", "auto"); err != nil {
		t.Fatalf("GenerateImage falhou: %v", err)
	}

	imgCfg := captured["image_config"].(map[string]interface{})
	if ratio, _ := imgCfg["aspect_ratio"].(string); ratio != "auto" {
		t.Errorf("esperado aspect_ratio 'auto', obtido '%v'", imgCfg["aspect_ratio"])
	}
}
