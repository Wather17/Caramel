package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListModelsContextRetriesTransientResponses(t *testing.T) {
	oldURL := OpenRouterModelsURL
	oldBackoff := retryBackoffBase
	t.Cleanup(func() {
		OpenRouterModelsURL = oldURL
		retryBackoffBase = oldBackoff
	})
	retryBackoffBase = 0

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			http.Error(w, "temporariamente indisponível", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"text/model","pricing":{"prompt":"0.000001"}}],"links":{"next":""}}`)
	}))
	defer server.Close()
	OpenRouterModelsURL = server.URL + "/api/v1/models"

	models, err := ListModelsContext(context.Background())
	if err != nil {
		t.Fatalf("ListModelsContext deveria recuperar falhas transitórias: %v", err)
	}
	if len(models) != 1 || models[0].PromptPrice != 0.000001 {
		t.Fatalf("resposta recuperada inesperada: %+v", models)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("esperava três tentativas, obtido %d", got)
	}
}

func TestListModelsContextInterrompePaginacaoExcessiva(t *testing.T) {
	oldURL := OpenRouterModelsURL
	t.Cleanup(func() { OpenRouterModelsURL = oldURL })
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		fmt.Fprintf(w, `{"data":[],"links":{"next":"/api/v1/models?offset=%d&limit=500"}}`, offset+1)
	}))
	defer server.Close()
	OpenRouterModelsURL = server.URL + "/api/v1/models"

	_, err := ListModelsContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "100 páginas") {
		t.Fatalf("esperava erro de limite de paginação, obtido: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 100 {
		t.Fatalf("deveria consultar no máximo 100 páginas, obtido %d", got)
	}
}

func TestListModelsContextCancelaRequisicaoEmAndamento(t *testing.T) {
	oldURL := OpenRouterModelsURL
	t.Cleanup(func() { OpenRouterModelsURL = oldURL })
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()
	OpenRouterModelsURL = server.URL + "/api/v1/models"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := ListModelsContext(ctx)
		result <- err
	}()
	select {
	case <-requestStarted:
		cancel()
	case <-ctx.Done():
		t.Fatal("requisição não deveria ser cancelada antes de iniciar")
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, obtido: %v", err)
	}
}
