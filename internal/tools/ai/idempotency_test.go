package ai_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"caramel/internal/tools/ai"
)

const idempotencyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestGenerateImageMarcaResultadoAmbiguoEm5xxSemRepetir(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if r.Header.Get("Idempotency-Key") == "" || r.Header.Get("X-Caramel-Correlation-ID") == "" {
			t.Error("requisição cobrada deveria enviar correlação")
		}
		http.Error(w, "gateway após aceitação", http.StatusBadGateway)
	}))
	defer server.Close()

	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	client, _ := ai.NewClient("sk-test")
	_, _, err := client.GenerateImage("bolo", "modelo", "1:1")
	var ambiguous *ai.UnknownOutcomeError
	if !errors.As(err, &ambiguous) || atomic.LoadInt32(&requests) != 1 {
		t.Fatalf("5xx após envio deveria ser ambíguo sem retry: requests=%d err=%v", requests, err)
	}
	if !strings.Contains(err.Error(), "unknown_outcome") || !strings.Contains(err.Error(), "não será repetida") {
		t.Fatalf("mensagem deveria orientar a não repetir: %v", err)
	}
}

func TestGenerateImageCorrelationERequestID(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("X-Request-ID", "req-123")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,%s"}}]}}]}`, idempotencyPNG)
	}))
	defer server.Close()

	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	var metadata []ai.RequestMetadata
	client, _ := ai.NewClient("sk-test")
	client.MetadataWriter = func(value ai.RequestMetadata) { metadata = append(metadata, value) }
	for i := 0; i < 2; i++ {
		if _, _, err := client.GenerateImage("bolo", "modelo", "1:1"); err != nil {
			t.Fatal(err)
		}
	}
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("correlação deveria ser estável e não vazia: %v", keys)
	}
	if len(metadata) != 2 || metadata[0].RequestID != "req-123" || metadata[0].CorrelationID != keys[0] || metadata[0].StatusCode != http.StatusOK {
		t.Fatalf("metadados inesperados: %+v", metadata)
	}
}

func TestGenerateImageRespostaPerdidaDepoisDeEnviar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("servidor de teste não suporta hijack")
		}
		conn, _, err := hijacker.Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer server.Close()

	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	client, _ := ai.NewClient("sk-test")
	_, _, err := client.GenerateImage("bolo", "modelo", "1:1")
	var ambiguous *ai.UnknownOutcomeError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("resposta perdida após envio deveria ser ambígua: %v", err)
	}
}
