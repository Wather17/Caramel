package ai_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"caramel/internal/tools/ai"
)

// tinyColorizedPNG é um PNG 1x1 vermelho usado como "imagem colorida" falsa retornada pelo mock
const tinyColorizedPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// writeSolidPNG grava um PNG sólido de uma cor em disco (helper local para testes do colorizer)
func writeSolidPNG(t *testing.T, name string, c color.RGBA) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, c)
		}
	}

	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("falha ao criar imagem de teste: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("falha ao codificar imagem de teste: %v", err)
	}
	return path
}

// bwDrawingPNG gera um PNG P&B (fundo branco, círculo preto) que simula uma ilustração colorível
func bwDrawingPNG(t *testing.T, name string) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	// "Desenho" preto central (quadrado oco simulando line art)
	for i := 15; i < 45; i++ {
		for _, p := range [][2]int{{i, 15}, {i, 44}, {15, i}, {44, i}} {
			img.Set(p[0], p[1], color.RGBA{A: 255})
		}
	}

	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("falha ao criar imagem de teste: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("falha ao codificar imagem de teste: %v", err)
	}
	return path
}

// setupColorizeMock cria um servidor mock que responde diferente para o modelo de triagem
// (resposta de texto JSON) e para o modelo de coloração (resposta com imagem data URL).
func setupColorizeMock(t *testing.T, triageStatus int, triageBody string) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("falha ao decodificar requisição no mock: %v", err)
		}

		model, _ := reqBody["model"].(string)

		// Requisição de triagem (modelo de visão barato)
		if model == ai.DefaultTriageModel {
			w.WriteHeader(triageStatus)
			fmt.Fprint(w, triageBody)
			return
		}

		// Requisição de coloração (modelo de geração): devolve um PNG via images[]
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"choices": [{
				"message": {
					"role": "assistant",
					"images": [{"type": "image_url", "image_url": {"url": "data:image/png;base64,%s"}}]
				}
			}]
		}`, tinyColorizedPNG)
	}))

	original := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() {
		ai.OpenRouterAPIURL = original
		server.Close()
	})
}

func baseOptions(apiKey, outDir string) ai.ColorizeOptions {
	return ai.ColorizeOptions{
		OutputDir:   outDir,
		APIKey:      apiKey,
		Model:       ai.DefaultModel,
		TriageModel: ai.DefaultTriageModel,
	}
}

func TestColorizeSingleImage_SkipLocalImagemJaColorida(t *testing.T) {
	// Imagem totalmente vermelha: deve ser pulada pela camada LOCAL sem nenhuma chamada HTTP.
	// (Não configuramos mock aqui de propósito: qualquer chamada de rede falharia e invalidaria o teste.)
	imgPath := writeSolidPNG(t, "colorida.png", color.RGBA{R: 230, G: 30, B: 30, A: 255})
	outDir := t.TempDir()

	res, err := ai.ColorizeSingleImage(imgPath, baseOptions("sk-test", outDir))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !res.Skipped {
		t.Fatal("esperado Skipped=true para imagem já colorida")
	}
	if res.SkipStage != "local" {
		t.Errorf("esperado SkipStage='local', obtido '%s'", res.SkipStage)
	}
	if res.SkipReason == "" {
		t.Error("esperado SkipReason preenchido")
	}
}

func TestColorizeSingleImage_SkipTriageLLM(t *testing.T) {
	// Imagem P&B: passa na camada local, mas a LLM rejeita (ex: "apenas texto")
	setupColorizeMock(t, http.StatusOK, `{
		"choices": [{"message": {"role": "assistant", "content": "{\"should_colorize\": false, \"reason\": \"imagem contém apenas texto\"}"}}]
	}`)

	imgPath := bwDrawingPNG(t, "texto.png")
	outDir := t.TempDir()

	res, err := ai.ColorizeSingleImage(imgPath, baseOptions("sk-test", outDir))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !res.Skipped {
		t.Fatal("esperado Skipped=true quando a LLM de triagem rejeita")
	}
	if res.SkipStage != "triage" {
		t.Errorf("esperado SkipStage='triage', obtido '%s'", res.SkipStage)
	}
	if res.SkipReason != "imagem contém apenas texto" {
		t.Errorf("SkipReason inesperado: %q", res.SkipReason)
	}

	// Nenhuma imagem colorida deve ter sido salva
	entries, _ := os.ReadDir(outDir)
	if len(entries) != 0 {
		t.Errorf("nenhum arquivo deveria ter sido salvo, encontrados: %d", len(entries))
	}
}

func TestColorizeSingleImage_FailOpenNaTriagem(t *testing.T) {
	// A triagem falha (429 rate limit do free tier): deve colorir mesmo assim (fail-open)
	setupColorizeMock(t, http.StatusTooManyRequests, `{"error": {"message": "rate limit", "code": 429}}`)

	imgPath := bwDrawingPNG(t, "desenho.png")
	outDir := t.TempDir()

	res, err := ai.ColorizeSingleImage(imgPath, baseOptions("sk-test", outDir))
	if err != nil {
		t.Fatalf("erro inesperado no fail-open: %v", err)
	}
	if res.Skipped {
		t.Error("esperado Skipped=false no fail-open (triagem com erro deve colorir)")
	}
	if res.ColorizedPath == "" {
		t.Fatal("esperado ColorizedPath preenchido após coloração fail-open")
	}
	if _, err := os.Stat(res.ColorizedPath); err != nil {
		t.Errorf("arquivo colorido não foi salvo em disco: %v", err)
	}
}

func TestColorizeSingleImage_AprovadaPelaTriagem(t *testing.T) {
	// Caminho feliz: imagem P&B aprovada pela triagem → colorida e salva
	setupColorizeMock(t, http.StatusOK, `{
		"choices": [{"message": {"role": "assistant", "content": "{\"should_colorize\": true, \"reason\": \"ilustração em preto e branco\"}"}}]
	}`)

	imgPath := bwDrawingPNG(t, "gato.png")
	outDir := t.TempDir()

	res, err := ai.ColorizeSingleImage(imgPath, baseOptions("sk-test", outDir))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if res.Skipped {
		t.Errorf("esperado Skipped=false, obtido true (motivo: %s)", res.SkipReason)
	}
	if res.ColorizedPath == "" {
		t.Fatal("esperado ColorizedPath preenchido")
	}

	// Valida que o arquivo salvo é a imagem retornada pelo mock
	saved, err := os.ReadFile(res.ColorizedPath)
	if err != nil {
		t.Fatalf("falha ao ler imagem salva: %v", err)
	}
	expected, _ := base64.StdEncoding.DecodeString(tinyColorizedPNG)
	if len(saved) != len(expected) {
		t.Errorf("tamanho da imagem salva (%d) difere do esperado (%d)", len(saved), len(expected))
	}
}

func TestColorizeSingleImage_TriagemDesativada(t *testing.T) {
	// Com DisableTriage=true, mesmo imagem colorida deve ir direto para a coloração
	setupColorizeMock(t, http.StatusInternalServerError, `{"error": {"message": "não deveria ser chamado", "code": 500}}`)

	imgPath := writeSolidPNG(t, "colorida.png", color.RGBA{R: 230, G: 30, B: 30, A: 255})
	outDir := t.TempDir()

	opts := baseOptions("sk-test", outDir)
	opts.DisableTriage = true

	res, err := ai.ColorizeSingleImage(imgPath, opts)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if res.Skipped {
		t.Error("esperado Skipped=false com triagem desativada")
	}
	if res.ColorizedPath == "" {
		t.Error("esperado coloração direta quando DisableTriage=true")
	}
}

func TestColorizeSingleImage_SemAPIKey(t *testing.T) {
	_, err := ai.ColorizeSingleImage("qualquer.png", baseOptions("", t.TempDir()))
	if err == nil {
		t.Error("esperado erro ao executar sem chave de API")
	}
}
