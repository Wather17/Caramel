package ai

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryableError marca erros transitórios (429, 5xx, falha de rede) que podem
// ser reexecutados com backoff. Erros permanentes (400, 401, 422...) não são
// retryable — retentá-los desperdiçaria chamadas e dinheiro.
type retryableError struct {
	err           error
	statusCode    int
	retryAfter    time.Duration
	hasRetryAfter bool
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// StatusCode retorna o status HTTP associado ao erro, quando disponível.
func (e *retryableError) StatusCode() int {
	if e == nil {
		return 0
	}
	return e.statusCode
}

// RetryAfter retorna o atraso indicado pelo provedor quando ele foi válido e
// ficou dentro do teto de segurança configurado.
func (e *retryableError) RetryAfter() (time.Duration, bool) {
	if e == nil || !e.hasRetryAfter {
		return 0, false
	}
	return e.retryAfter, true
}

// isRetryable indica se o erro é transitório e pode ser reexecutado
func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}

// retryBackoffMax limita o atraso calculado por backoff e Retry-After. Valores
// maiores ou inválidos usam o backoff local para evitar bloquear a execução.
var retryBackoffMax = 30 * time.Second

var retryNow = time.Now

// statusError monta a mensagem de erro de um status HTTP não-200, truncando o
// corpo (evita vazar conteúdo sensível/huge em logs) e marcando como retryable
// quando o status indica falha transitória (408, 429 ou 5xx).
func statusError(status int, body []byte) error {
	return statusErrorWithHeaders(status, nil, body)
}

func statusErrorWithHeaders(status int, headers http.Header, body []byte) error {
	msg := fmt.Sprintf("API OpenRouter retornou status %d", status)
	if len(body) > 0 {
		msg += ": " + truncateForError(string(body))
	}

	if status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError {
		retryErr := &retryableError{err: fmt.Errorf("%s", msg), statusCode: status}
		if headers != nil {
			if delay, ok := parseRetryAfter(headers.Get("Retry-After"), retryNow()); ok {
				retryErr.retryAfter = delay
				retryErr.hasRetryAfter = true
			}
		}
		return retryErr
	}
	return fmt.Errorf("%s", msg)
}

func parseRetryAfter(raw string, now time.Time) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if seconds < 0 || seconds > int64(retryBackoffMax/time.Second) {
			return 0, false
		}
		delay := time.Duration(seconds) * time.Second
		if delay > retryBackoffMax {
			return 0, false
		}
		return delay, true
	}
	deadline, err := http.ParseTime(raw)
	if err != nil {
		return 0, false
	}
	delay := deadline.Sub(now)
	if delay < 0 || delay > retryBackoffMax {
		return 0, false
	}
	return delay, true
}

func apiError(message string, statusCode int) error {
	err := fmt.Errorf("%s", message)
	if statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError {
		return &retryableError{err: err, statusCode: statusCode}
	}
	return err
}

// retryBackoffBase é a base do backoff exponencial; declarado como var para
// permitir redução em testes (evita sleeps longos).
var retryBackoffBase = 1200 * time.Millisecond

// retryWithBackoff executa fn até obter sucesso ou esgotar as tentativas,
// retentando apenas erros transitórios (isRetryable) com backoff exponencial
// + jitter para evitar rajadas sincronizadas.
func retryWithBackoff(maxRetries int, fn func() error) error {
	return retryWithBackoffContext(context.Background(), maxRetries, fn)
}

// retryWithBackoffContext is the context-aware variant of retryWithBackoff.
// Cancellation interrupts the backoff immediately and is returned to the caller.
func retryWithBackoffContext(ctx context.Context, maxRetries int, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err = fn()
		if err == nil {
			return nil
		}
		if !isRetryable(err) || attempt == maxRetries {
			return err
		}

		delay := retryDelay(err, attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

func retryDelay(err error, attempt int) time.Duration {
	var retryErr *retryableError
	if errors.As(err, &retryErr) {
		if delay, ok := retryErr.RetryAfter(); ok {
			return delay
		}
	}
	backoff := time.Duration(attempt) * retryBackoffBase
	if backoff < 0 {
		backoff = 0
	}
	jitter := time.Duration(rand.Intn(400)) * time.Millisecond
	delay := backoff + jitter
	if delay > retryBackoffMax {
		return retryBackoffMax
	}
	return delay
}
