package ai_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestColorizeImageErrorBranches(t *testing.T) {
	// PNG válido em arquivo temporário para o encode da requisição
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("falha ao gerar png: %v", err)
	}
	imgPath := filepath.Join(t.TempDir(), "origem.png")
	if err := os.WriteFile(imgPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao salvar png: %v", err)
	}

	client, _ := ai.NewClient("sk-test")

	tests := []struct {
		name      string
		responder func(w http.ResponseWriter, r *http.Request)
		wantMsg   string
	}{
		{
			name: "erro http permanente",
			responder: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error":{"message":"sem créditos"}}`, http.StatusBadRequest)
			},
			wantMsg: "400",
		},
		{
			name: "erro http transitorio",
			responder: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "gateway", http.StatusBadGateway)
			},
			wantMsg: "502",
		},
		{
			name: "api devolve objeto de erro",
			responder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"error":{"message":"quota excedida"}}`)
			},
			wantMsg: "quota excedida",
		},
		{
			name: "resposta sem escolhas",
			responder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"choices":[]}`)
			},
			wantMsg: "resposta vazia",
		},
		{
			name: "conteudo textual sem imagem",
			responder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"desculpe, nao sei desenhar"}}]}`)
			},
			wantMsg: "falha ao extrair imagem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tt.responder))
			defer srv.Close()

			old := ai.OpenRouterAPIURL
			ai.OpenRouterAPIURL = srv.URL
			defer func() { ai.OpenRouterAPIURL = old }()

			_, _, err := client.ColorizeImage(imgPath, "colorize", "m/imagem")
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("esperado erro contendo %q, obtido: %v", tt.wantMsg, err)
			}
		})
	}
}
