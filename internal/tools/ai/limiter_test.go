package ai_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"caramel/internal/tools/ai"
)

func TestClientRequestLimiterCapsConcurrentRequests(t *testing.T) {
	var inFlight int32
	var maxInFlight int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			previous := atomic.LoadInt32(&maxInFlight)
			if current <= previous || atomic.CompareAndSwapInt32(&maxInFlight, previous, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="}}]}}]}`)
	}))
	defer server.Close()

	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })

	client, err := ai.NewClient("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	client.SetMaxConcurrentRequests(2)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := client.GenerateImage("teste", "modelo", "1:1")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("request deveria concluir: %v", err)
		}
	}
	if observed := atomic.LoadInt32(&maxInFlight); observed > 2 {
		t.Fatalf("limitador permitiu %d requests simultâneos", observed)
	}
}
