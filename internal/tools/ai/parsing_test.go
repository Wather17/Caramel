package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// tinyPNGBase64 é um PNG 1x1 válido
const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestExtractImageBytes_DataURLLimpa(t *testing.T) {
	c := &Client{}
	got, ext, err := c.extractImageBytesFromResponse("data:image/png;base64," + tinyPNGBase64)
	if err != nil {
		t.Fatalf("erro ao extrair data URL limpa: %v", err)
	}
	if ext != "png" {
		t.Errorf("esperado png, obtido %s", ext)
	}
	if len(got) == 0 {
		t.Error("bytes vazios")
	}
}

func TestExtractImageBytes_JSONEscapado(t *testing.T) {
	c := &Client{}
	// Aspa escapada de JSON (\" ) após a data URL
	content := `data:image/png;base64,` + tinyPNGBase64 + `\"`
	got, ext, err := c.extractImageBytesFromResponse(content)
	if err != nil {
		t.Fatalf("erro ao extrair data URL com escape JSON: %v", err)
	}
	if ext != "png" || len(got) == 0 {
		t.Errorf("extração falhou: ext=%q len=%d", ext, len(got))
	}
}

func TestExtractImageBytes_DentroDeArrayJSON(t *testing.T) {
	c := &Client{}
	// Resposta de conteúdo multimodal (array) — primeiro item é texto, segundo é imagem
	content := `[{"type":"text","text":"ok"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + tinyPNGBase64 + `"}}]`
	got, ext, err := c.extractImageBytesFromResponse(content)
	if err != nil {
		t.Fatalf("erro ao extrair imagem de array JSON: %v", err)
	}
	if ext != "png" || len(got) == 0 {
		t.Errorf("extração falhou: ext=%q len=%d", ext, len(got))
	}
}

func TestExtractImageBytes_MultiplasDataURLs(t *testing.T) {
	c := &Client{}
	// Primeira ocorrência é uma "imagem" inválida (base64 de texto), segunda é PNG válido
	invalid := base64TextBytes()
	content := "data:image/png;base64," + invalid + " data:image/jpeg;base64," + tinyJPEGBase64()
	got, ext, err := c.extractImageBytesFromResponse(content)
	if err != nil {
		t.Fatalf("erro ao extrair segunda data URL: %v", err)
	}
	if ext != "jpg" || len(got) == 0 {
		t.Errorf("esperado jpg da segunda URL, obtido ext=%q len=%d", ext, len(got))
	}
}

func TestExtractImageBytes_RejeitaLixo(t *testing.T) {
	c := &Client{}
	// Base64 de um texto qualquer (não é imagem) deve ser rejeitado
	_, _, err := c.extractImageBytesFromResponse("data:image/png;base64," + base64TextBytes())
	if err == nil {
		t.Error("esperava erro ao extrair base64 que não é imagem")
	}
}

func TestExtractImageBytes_Base64PuroFallback(t *testing.T) {
	c := &Client{}
	got, ext, err := c.extractImageBytesFromResponse(tinyPNGBase64)
	if err != nil {
		t.Fatalf("erro no fallback base64 puro: %v", err)
	}
	if ext != "png" || len(got) == 0 {
		t.Errorf("fallback falhou: ext=%q len=%d", ext, len(got))
	}
}

func TestExtractImageBytes_URLCaseInsensitive(t *testing.T) {
	c := &Client{}
	got, ext, err := c.extractImageBytesFromResponse("DATA:IMAGE/PNG;BASE64," + tinyPNGBase64)
	if err != nil {
		t.Fatalf("erro ao extrair data URL em caixa alta: %v", err)
	}
	if ext != "png" || len(got) == 0 {
		t.Errorf("caixa alta falhou: ext=%q len=%d", ext, len(got))
	}
}

func TestDownloadImageFromURL_OKAndMagicBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3})
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client()}
	got, ext, err := c.downloadImageFromURL(srv.URL)
	if err != nil {
		t.Fatalf("download falhou: %v", err)
	}
	if ext != "png" || len(got) == 0 {
		t.Errorf("download falhou: ext=%q len=%d", ext, len(got))
	}
}

func TestDownloadImageFromURL_RejeitaNaoImagem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>not an image</html>"))
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client()}
	if _, _, err := c.downloadImageFromURL(srv.URL); err == nil {
		t.Error("esperava erro ao baixar conteúdo que não é imagem")
	}
}

func TestDownloadImageFromURL_LimiteDeTamanho(t *testing.T) {
	big := []byte(strings.Repeat("A", 26*1024*1024)) // 26 MB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(big)
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client()}
	if _, _, err := c.downloadImageFromURL(srv.URL); err == nil {
		t.Error("esperava erro ao baixar imagem acima do limite")
	}
}

func TestDownloadImageFromURL_StatusErro(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client()}
	if _, _, err := c.downloadImageFromURL(srv.URL); err == nil {
		t.Error("esperava erro para status 404")
	}
}

// helpers de fixtures

func base64TextBytes() string {
	// "isto não é uma imagem" codificado — decodifica para texto, não para PNG
	return "aXN0byBuw6NvIMOpIHVtYSBpbWFnZW0="
}

func tinyJPEGBase64() string {
	// JPEG 1x1 válido
	return "/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////2wBDAf//////////////////////////////////////////////////////////////////////////////////////wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAX/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAH/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oACAEBAAEFAqf/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oACAEDAQE/AV//xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oACAECAQE/AV//xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oACAEBAAY/Aqf/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oACAEBAAE/IV//2gAMAwEAAgADAAAAEP/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8QV//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQIBAT8QV//EABQQAQAAAAAAAAAAAAAAAAAAABD/2gAIAQEAAT8QV//Z"
}