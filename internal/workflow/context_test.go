package workflow

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"caramel/internal/tools/ai"
	"caramel/internal/vault"
	"caramel/internal/workspace"
)

func TestImageServiceGeneratePropagaCancelamentoNaSintese(t *testing.T) {
	t.Setenv("CARAMEL_WORKSPACE_DIR", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	project, err := workspace.CreateProject("Cancelamento")
	if err != nil {
		t.Fatal(err)
	}

	var requests atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRequest := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseRequest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		close(started)
		<-release
	}))
	defer server.Close()
	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, runErr := (&ImageService{Project: project}).Generate(ctx, ImageOptions{Items: []string{"bolo"}})
		resultCh <- runErr
	}()
	select {
	case <-started:
		cancel()
		releaseRequest()
	case <-time.After(5 * time.Second):
	}

	select {
	case runErr := <-resultCh:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("cancelamento deveria ser preservado: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("síntese não foi interrompida após cancelamento")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("cancelamento na síntese não deveria iniciar geração: requests=%d", got)
	}
	reopened, err := workspace.OpenProject(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Runs) != 1 || reopened.Runs[0].Status != "canceled" {
		t.Fatalf("execução deveria terminar cancelada: %+v", reopened.Runs)
	}
}

func TestVaultImageServiceGeneratePropagaCancelamentoNaSintese(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	v, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	var requests atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRequest := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseRequest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		close(started)
		<-release
	}))
	defer server.Close()
	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, runErr := (&VaultImageService{Vault: v}).Generate(ctx, ImageOptions{Items: []string{"bolo"}})
		resultCh <- runErr
	}()
	select {
	case <-started:
		cancel()
		releaseRequest()
	case <-time.After(5 * time.Second):
	}

	select {
	case runErr := <-resultCh:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("cancelamento deveria ser preservado: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("síntese do vault não foi interrompida após cancelamento")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("cancelamento na síntese do vault não deveria iniciar geração: requests=%d", got)
	}
}
