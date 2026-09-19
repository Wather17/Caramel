package ai

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateImageRejeitaCorpoGrandeSemRetry(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"`+strings.Repeat("A", int(DefaultMaxResponseBodyBytes))+`"}}]}`)
	}))
	defer server.Close()

	oldURL := OpenRouterAPIURL
	OpenRouterAPIURL = server.URL
	t.Cleanup(func() { OpenRouterAPIURL = oldURL })

	client := &Client{APIKey: "sk-test", HTTPClient: server.Client()}
	_, _, err := client.GenerateImage("teste", "modelo", "1:1")
	var limitErr *ResourceLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("esperava erro de limite de recurso: %T %v", err, err)
	}
	if isRetryable(err) || requests != 1 {
		t.Fatalf("erro de recurso não deveria repetir: retryable=%v requests=%d", isRetryable(err), requests)
	}
}

func TestValidateImageURLBloqueiaEsquemaEIPsPrivados(t *testing.T) {
	if err := validateImageURL(context.Background(), "http://example.com/image.png", ImageURLPolicy{}); err == nil {
		t.Fatal("HTTP deveria ser rejeitado por padrão")
	}
	if err := validateImageURL(context.Background(), "https://127.0.0.1/image.png", ImageURLPolicy{}); err == nil {
		t.Fatal("loopback deveria ser rejeitado")
	}
	policy := ImageURLPolicy{Resolve: func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.1.10")}, nil
	}}
	if err := validateImageURL(context.Background(), "https://public.example/image.png", policy); err == nil {
		t.Fatal("DNS resolvendo para RFC1918 deveria ser rejeitado")
	}
}

func TestDownloadImageFromURLLimitaRedirects(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/one", http.StatusFound)
		case "/one":
			http.Redirect(w, r, "/two", http.StatusFound)
		case "/two":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		URLPolicy:  ImageURLPolicy{AllowPrivateNetworks: true, MaxRedirects: 1},
	}
	_, _, err := client.downloadImageFromURL(server.URL + "/start")
	if err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Fatalf("esperava limite de redirects, obtido %v", err)
	}
}

func TestDownloadImageFromURLHTTPSControlado(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1})
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), URLPolicy: ImageURLPolicy{AllowPrivateNetworks: true}}
	got, ext, err := client.downloadImageFromURL(server.URL + "/image.png")
	if err != nil || ext != "png" || len(got) == 0 {
		t.Fatalf("URL HTTPS controlada deveria funcionar: ext=%q len=%d err=%v", ext, len(got), err)
	}
}

func TestExtractImageBytesRejeitaMIMEDataURLNaoSuportado(t *testing.T) {
	client := &Client{}
	_, _, err := client.extractImageBytesFromResponse("data:image/gif;base64," + tinyPNGBase64)
	if err == nil || !strings.Contains(err.Error(), "MIME") {
		t.Fatalf("MIME incompatível deveria falhar com mensagem acionável: %v", err)
	}
}

func TestResourceLimitErrorMensagem(t *testing.T) {
	err := (&ResourceLimitError{Resource: "resposta", Limit: 10}).Error()
	if !strings.Contains(err, "10 bytes") || !strings.Contains(err, "resposta") {
		t.Fatalf("mensagem inesperada: %s", err)
	}
}
