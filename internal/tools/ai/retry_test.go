package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStatusError_RetryableAndTruncation(t *testing.T) {
	// 429 e 5xx são transitórios (retryable)
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway} {
		err := statusError(status, []byte("corpo"))
		if !isRetryable(err) {
			t.Errorf("status %d deveria ser retryable", status)
		}
	}

	// 400, 401, 422 são permanentes (não retryable)
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusUnprocessableEntity} {
		err := statusError(status, []byte("corpo"))
		if isRetryable(err) {
			t.Errorf("status %d não deveria ser retryable", status)
		}
	}

	// Corpo longo é truncado na mensagem de erro
	longBody := []byte(strings.Repeat("A", 5000))
	err := statusError(http.StatusBadGateway, longBody)
	if len(err.Error()) > 400 {
		t.Errorf("mensagem de erro deveria truncar o corpo (len=%d)", len(err.Error()))
	}
}

func TestRetryAfterAceitaSegundosEDataHTTP(t *testing.T) {
	oldMax := retryBackoffMax
	retryBackoffMax = 30 * time.Second
	t.Cleanup(func() { retryBackoffMax = oldMax })

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if delay, ok := parseRetryAfter("3", now); !ok || delay != 3*time.Second {
		t.Fatalf("Retry-After em segundos inesperado: delay=%v ok=%v", delay, ok)
	}
	date := now.Add(5 * time.Second).Format(http.TimeFormat)
	if delay, ok := parseRetryAfter(date, now); !ok || delay != 5*time.Second {
		t.Fatalf("Retry-After em data HTTP inesperado: delay=%v ok=%v", delay, ok)
	}
	for _, raw := range []string{"-1", "31", "inválido", now.Add(-time.Second).Format(http.TimeFormat)} {
		if _, ok := parseRetryAfter(raw, now); ok {
			t.Errorf("Retry-After inválido deveria ser rejeitado: %q", raw)
		}
	}
}

func TestStatusErrorPreservaRetryAfter(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Retry-After", "3")
	err := statusErrorWithHeaders(http.StatusTooManyRequests, headers, nil)
	re, ok := err.(*retryableError)
	if !ok {
		t.Fatalf("esperava retryableError, obtido %T", err)
	}
	if re.StatusCode() != http.StatusTooManyRequests {
		t.Errorf("status inesperado: %d", re.StatusCode())
	}
	if delay, ok := re.RetryAfter(); !ok || delay != 3*time.Second {
		t.Errorf("Retry-After não preservado: delay=%v ok=%v", delay, ok)
	}
}

func TestRetryWithBackoffRespeitaRetryAfter(t *testing.T) {
	calls := 0
	started := time.Now()
	err := retryWithBackoffContext(context.Background(), 2, func() error {
		calls++
		if calls == 1 {
			return &retryableError{err: fmt.Errorf("limite"), retryAfter: 20 * time.Millisecond, hasRetryAfter: true}
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("retry deveria concluir após Retry-After: calls=%d err=%v", calls, err)
	}
	if elapsed := time.Since(started); elapsed < 15*time.Millisecond {
		t.Fatalf("Retry-After não foi respeitado: %v", elapsed)
	}
}

func TestRetryableErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("conexão resetada")
	wrapped := fmt.Errorf("falha ao colorir: %w", &retryableError{err: inner})

	if !strings.Contains(wrapped.Error(), "conexão resetada") {
		t.Errorf("Unwrap deveria expor o erro original, obtido: %v", wrapped)
	}

	re, ok := statusError(http.StatusServiceUnavailable, []byte("lento")).(*retryableError)
	if !ok {
		t.Fatal("503 deveria produzir retryableError")
	}
	if re.Unwrap() == nil || !strings.Contains(re.Unwrap().Error(), "503") {
		t.Errorf("Unwrap do retryableError deveria preservar a mensagem de status, obtido: %v", re.Unwrap())
	}
}

func TestRetryWithBackoff_OnlyRetriesTransient(t *testing.T) {
	old := retryBackoffBase
	retryBackoffBase = time.Millisecond
	defer func() { retryBackoffBase = old }()

	// Caso 1: falha transitória 2x e depois sucesso -> 3 tentativas
	transientCalls := 0
	err := retryWithBackoff(3, func() error {
		transientCalls++
		if transientCalls < 3 {
			return &retryableError{err: fmt.Errorf("transitório %d", transientCalls)}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("esperava sucesso após retries transitórios: %v", err)
	}
	if transientCalls != 3 {
		t.Errorf("esperado 3 chamadas, obtido %d", transientCalls)
	}

	// Caso 2: erro permanente não é retentado
	permanentCalls := 0
	err = retryWithBackoff(3, func() error {
		permanentCalls++
		return fmt.Errorf("erro permanente")
	})
	if err == nil {
		t.Fatal("esperava erro permanente")
	}
	if permanentCalls != 1 {
		t.Errorf("erro permanente não deveria ser retentado: %d chamadas", permanentCalls)
	}
}

func TestRetryWithBackoffContextCancellationInterruptsWait(t *testing.T) {
	old := retryBackoffBase
	retryBackoffBase = time.Second
	defer func() { retryBackoffBase = old }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})

	go func() {
		<-started
		cancel()
	}()

	startedAt := time.Now()
	err := retryWithBackoffContext(ctx, 3, func() error {
		close(started)
		return &retryableError{err: fmt.Errorf("temporário")}
	})
	if err != context.Canceled {
		t.Fatalf("esperava context.Canceled, obtido: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 200*time.Millisecond {
		t.Fatalf("cancelamento deveria interromper o backoff rapidamente, levou %v", elapsed)
	}
}

func TestDetectImageType(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0}
	if ext, ok := detectImageType(png); !ok || ext != "png" {
		t.Errorf("PNG não detectado: ext=%q ok=%v", ext, ok)
	}

	jpg := []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0}
	if ext, ok := detectImageType(jpg); !ok || ext != "jpg" {
		t.Errorf("JPEG não detectado: ext=%q ok=%v", ext, ok)
	}

	webp := []byte("RIFF\x10\x00\x00\x00WEBPVP8 ")
	if ext, ok := detectImageType(webp); !ok || ext != "webp" {
		t.Errorf("WEBP não detectado: ext=%q ok=%v", ext, ok)
	}

	if _, ok := detectImageType([]byte("não é imagem")); ok {
		t.Error("texto não deveria ser detectado como imagem")
	}
	if _, ok := detectImageType(nil); ok {
		t.Error("nil não deveria ser detectado como imagem")
	}
}
