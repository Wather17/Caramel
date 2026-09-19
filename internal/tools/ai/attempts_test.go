package ai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"caramel/internal/tools/ai"
)

const attemptsPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestAttemptCollectorRedactsAndOrdersEvents(t *testing.T) {
	collector := ai.NewAttemptCollector()
	collector.Add(ai.AttemptMetadata{Operation: "generate", ItemName: " bolo\n", CorrelationID: "caramel-safe", Status: "success"})
	collector.Add(ai.AttemptMetadata{Operation: "generate", ErrorClass: "permanent", Status: "failure"})
	events := collector.Snapshot()
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Sequence != 2 || strings.Contains(events[0].ItemName, "\n") {
		t.Fatalf("eventos não foram redigidos/ordenados: %+v", events)
	}
	data, err := json.Marshal(events)
	if err != nil || strings.Contains(string(data), "prompt") {
		t.Fatalf("serialização não deveria conter prompt: %s err=%v", data, err)
	}
}

func TestGenerateImagePersisteUsoERequestIDNoEvento(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req-safe-1")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14,"cost":0.0042},"choices":[{"message":{"images":[{"image_url":{"url":"data:image/png;base64,%s"}}]}}]}`, attemptsPNG)
	}))
	defer server.Close()
	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	var event ai.AttemptMetadata
	client, _ := ai.NewClient("sk-test")
	client.AttemptWriter = func(value ai.AttemptMetadata) { event = value }
	if _, _, err := client.GenerateImage("bolo", "modelo", "1:1"); err != nil {
		t.Fatal(err)
	}
	if event.Status != "success" || event.StatusCode != http.StatusOK || event.RequestID != "req-safe-1" || event.Usage.TotalTokens != 14 || event.Usage.Cost != 0.0042 {
		t.Fatalf("metadados inesperados: %+v", event)
	}
	if event.CorrelationID == "" || strings.Contains(event.CorrelationID, "bolo") {
		t.Fatalf("correlação deveria ser redigida: %+v", event)
	}
}

func TestAnalyzeRoutineRegistraCadaRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "temporario", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"[{\"name\":\"bolo\",\"prompt\":\"desenho de bolo\"}]"}}]}`)
	}))
	defer server.Close()
	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	var events []ai.AttemptMetadata
	client, _ := ai.NewClient("sk-test")
	client.AttemptWriter = func(value ai.AttemptMetadata) { events = append(events, value) }
	if _, err := ai.SynthesizePromptsContext(nil, ai.HarnessConfig{Items: []string{"bolo"}, TextModel: "modelo"}, client); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].ErrorClass != "transient" || events[1].Status != "success" || events[0].Attempt != 1 || events[1].Attempt != 2 {
		t.Fatalf("tentativas não registradas: calls=%d events=%+v", calls.Load(), events)
	}
}
