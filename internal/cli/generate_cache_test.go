package cli

import (
	"bytes"
	"context"
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
	"strings"
	"sync"
	"testing"

	"caramel/internal/tools/ai"
	"caramel/internal/tools/cards"
	"caramel/internal/tools/pdf"
	"caramel/internal/vault"
)

func TestExecuteImageGenerationCachesDirectItemsAndAllowsOfflineHits(t *testing.T) {
	server, calls := imageCacheAPIMock(t, nil)
	defer server.Close()

	cache := openImageCacheVault(t)
	targetOne := filepath.Join(t.TempDir(), "first-output")
	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	items, warnings, err := executeImageGeneration(context.Background(), []string{"bolo"}, "", imageCacheHarnessConfig(targetOne, []string{"bolo"}), targetOne, cache, false, client, nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("primeira geração falhou: warnings=%v err=%v", warnings, err)
	}
	if len(items) != 1 || items[0].Status != "done" || items[0].Reused {
		t.Fatalf("primeiro resultado deveria ser gerado: %+v", items)
	}
	if calls.text() != 1 || calls.image() != 1 {
		t.Fatalf("esperava uma chamada de texto e uma de imagem, obtido text=%d image=%d", calls.text(), calls.image())
	}

	targetTwo := filepath.Join(t.TempDir(), "second-output")
	items, warnings, err = executeImageGeneration(context.Background(), []string{" Bolo "}, "", imageCacheHarnessConfig(targetTwo, []string{" Bolo "}), targetTwo, cache, false, nil, nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("hit offline falhou: warnings=%v err=%v", warnings, err)
	}
	if len(items) != 1 || items[0].Status != "done" || !items[0].Reused {
		t.Fatalf("segundo resultado deveria ser reutilizado: %+v", items)
	}
	if filepath.Dir(items[0].ImagePath) != targetTwo {
		t.Fatalf("imagem reutilizada não foi copiada para a saída: %s", items[0].ImagePath)
	}
	if err := cards.GenerateCardsHTML([]cards.CardItem{{Name: items[0].Name, ImagePath: items[0].ImagePath}}, filepath.Join(targetTwo, "cards.html"), cards.DefaultOptions()); err != nil {
		t.Fatalf("imagem reutilizada deveria funcionar nas fichas: %v", err)
	}
	if err := pdf.Generate2UpPDF([]string{items[0].ImagePath}, filepath.Join(targetTwo, "2up.pdf"), pdf.DefaultOptions()); err != nil {
		t.Fatalf("imagem reutilizada deveria funcionar no PDF 2-up: %v", err)
	}
	if calls.text() != 1 || calls.image() != 1 {
		t.Fatalf("hit local chamou a API: text=%d image=%d", calls.text(), calls.image())
	}
	encoded, err := json.Marshal(items[0])
	if err != nil || !strings.Contains(string(encoded), `"reused":true`) {
		t.Fatalf("resultado JSON deveria marcar a reutilização: %s err=%v", encoded, err)
	}
}

func TestExecuteImageGenerationRefreshesAndPreservesPreviousLibraryFile(t *testing.T) {
	server, calls := imageCacheAPIMock(t, [][]byte{solidPNG(20), solidPNG(220)})
	defer server.Close()
	cache := openImageCacheVault(t)
	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	firstDir := filepath.Join(t.TempDir(), "first")
	first, _, err := executeImageGeneration(context.Background(), []string{"bolo"}, "", imageCacheHarnessConfig(firstDir, []string{"bolo"}), firstDir, cache, false, client, nil)
	if err != nil {
		t.Fatal(err)
	}
	key := imageCacheKey("bolo", "", imageCacheHarnessConfig(firstDir, []string{"bolo"}))
	previous, found, err := cache.LookupGeneratedImage(context.Background(), key)
	if err != nil || !found {
		t.Fatalf("imagem inicial não entrou na biblioteca: found=%v err=%v", found, err)
	}

	refreshDir := filepath.Join(t.TempDir(), "refresh")
	refreshed, warnings, err := executeImageGeneration(context.Background(), []string{"bolo"}, "", imageCacheHarnessConfig(refreshDir, []string{"bolo"}), refreshDir, cache, true, client, nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("refresh falhou: warnings=%v err=%v", warnings, err)
	}
	latest, found, err := cache.LookupGeneratedImage(context.Background(), key)
	if err != nil || !found || latest.Path == previous.Path {
		t.Fatalf("refresh não atualizou a imagem ativa: latest=%+v previous=%+v err=%v", latest, previous, err)
	}
	if _, err := os.Stat(previous.Path); err != nil {
		t.Fatalf("refresh removeu a versão anterior: %v", err)
	}
	if len(refreshed) != 1 || refreshed[0].Reused || calls.text() != 2 || calls.image() != 2 || first[0].Reused {
		t.Fatalf("refresh deveria sintetizar e gerar novamente: first=%+v refreshed=%+v text=%d image=%d", first, refreshed, calls.text(), calls.image())
	}
}

func TestExecuteImageGenerationDeduplicatesAndCachesThemeItems(t *testing.T) {
	server, calls := imageCacheAPIMock(t, nil)
	defer server.Close()
	cache := openImageCacheVault(t)
	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "duplicates")
	items, warnings, err := executeImageGeneration(context.Background(), []string{"bolo", " Bolo "}, "", imageCacheHarnessConfig(dir, []string{"bolo", " Bolo "}), dir, cache, false, client, nil)
	if err != nil || len(warnings) != 0 || len(items) != 2 {
		t.Fatalf("geração duplicada falhou: items=%v warnings=%v err=%v", items, warnings, err)
	}
	if items[0].Reused || !items[1].Reused || calls.text() != 1 || calls.image() != 1 {
		t.Fatalf("chave duplicada gerou chamadas redundantes: items=%+v text=%d image=%d", items, calls.text(), calls.image())
	}

	themeDir := filepath.Join(t.TempDir(), "theme-one")
	themeCfg := imageCacheHarnessConfig(themeDir, nil)
	themeCfg.Count = 1
	themeFirst, warnings, err := executeImageGeneration(context.Background(), nil, "padaria", themeCfg, themeDir, cache, false, client, nil)
	if err != nil || len(warnings) != 0 || len(themeFirst) != 1 || themeFirst[0].Reused {
		t.Fatalf("primeira execução de tema falhou: items=%+v warnings=%v err=%v", themeFirst, warnings, err)
	}
	secondThemeDir := filepath.Join(t.TempDir(), "theme-two")
	themeCfg.OutputDir = secondThemeDir
	themeSecond, warnings, err := executeImageGeneration(context.Background(), nil, "padaria", themeCfg, secondThemeDir, cache, false, client, nil)
	if err != nil || len(warnings) != 0 || len(themeSecond) != 1 || !themeSecond[0].Reused {
		t.Fatalf("segunda execução de tema falhou: items=%+v warnings=%v err=%v", themeSecond, warnings, err)
	}
	if calls.text() != 3 || calls.image() != 2 {
		t.Fatalf("tema deveria sintetizar nas duas execuções e gerar imagem só uma vez: text=%d image=%d", calls.text(), calls.image())
	}
}

func TestExecuteImageGenerationContinuesWhenCacheLookupFails(t *testing.T) {
	server, calls := imageCacheAPIMock(t, nil)
	defer server.Close()
	cache := openImageCacheVault(t)
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "fallback")
	items, warnings, err := executeImageGeneration(context.Background(), []string{"bolo"}, "", imageCacheHarnessConfig(dir, []string{"bolo"}), dir, cache, false, client, nil)
	if err != nil || len(warnings) == 0 || len(items) != 1 || items[0].Status != "done" {
		t.Fatalf("falha de consulta deveria avisar e seguir: items=%+v warnings=%v err=%v", items, warnings, err)
	}
	if calls.text() != 1 || calls.image() != 1 {
		t.Fatalf("fallback deveria sintetizar e gerar: text=%d image=%d", calls.text(), calls.image())
	}
}

type imageCacheAPICalls struct {
	mu         sync.Mutex
	textCalls  int
	imageCalls int
}

func (c *imageCacheAPICalls) text() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.textCalls
}

func (c *imageCacheAPICalls) image() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.imageCalls
}

func imageCacheAPIMock(t *testing.T, imageResults [][]byte) (*httptest.Server, *imageCacheAPICalls) {
	t.Helper()
	previousURL := ai.OpenRouterAPIURL
	calls := &imageCacheAPICalls{}
	var nextImage int
	defaultImage := solidPNG(100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Modalities []string `json:"modalities"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("não foi possível ler requisição mock: %v", err)
			http.Error(w, "requisição inválida", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if len(request.Modalities) == 0 {
			calls.mu.Lock()
			calls.textCalls++
			calls.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `[{"name":"Bolo","slug":"01_bolo","prompt":"a pedagogical cake on white background"}]`}}}})
			return
		}
		calls.mu.Lock()
		calls.imageCalls++
		imageIndex := nextImage
		nextImage++
		calls.mu.Unlock()
		imageData := defaultImage
		if len(imageResults) > 0 {
			index := imageIndex
			if index >= len(imageResults) {
				index = len(imageResults) - 1
			}
			imageData = imageResults[index]
		}
		encoded := base64.StdEncoding.EncodeToString(imageData)
		response := fmt.Sprintf(`{"choices":[{"message":{"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,%s"}}]}}]}`, encoded)
		_, _ = w.Write([]byte(response))
	}))
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() {
		ai.OpenRouterAPIURL = previousURL
		server.Close()
	})
	return server, calls
}

func imageCacheHarnessConfig(outputDir string, items []string) ai.HarnessConfig {
	return ai.HarnessConfig{
		Items: items, Style: "clipart", TextModel: "text/model", ImageModel: "image/model",
		Aspect: "1:1", OutputDir: outputDir, MaxWorkers: 1,
	}
}

func openImageCacheVault(t *testing.T) *vault.Vault {
	t.Helper()
	t.Setenv("CARAMEL_VAULT_DIR", t.TempDir())
	cache, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	return cache
}

func solidPNG(value uint8) []byte {
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: value, G: 255 - value, B: value / 2, A: 255})
		}
	}
	_ = png.Encode(&buffer, img)
	return buffer.Bytes()
}
