package ai

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// retryableError marca erros transitórios (429, 5xx, falha de rede) que podem
// ser reexecutados com backoff. Erros permanentes (400, 401, 422...) não são
// retryable — retentá-los desperdiçaria chamadas e dinheiro.
type retryableError struct {
	err error
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// isRetryable indica se o erro é transitório e pode ser reexecutado
func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}

// statusError monta a mensagem de erro de um status HTTP não-200, truncando o
// corpo (evita vazar conteúdo sensível/huge em logs) e marcando como retryable
// quando o status indica falha transitória (429 ou 5xx).
func statusError(status int, body []byte) error {
	msg := fmt.Sprintf("API OpenRouter retornou status %d", status)
	if len(body) > 0 {
		msg += ": " + truncateForError(string(body))
	}

	if status == http.StatusTooManyRequests || status >= http.StatusInternalServerError {
		return &retryableError{err: fmt.Errorf("%s", msg)}
	}
	return fmt.Errorf("%s", msg)
}

// retryBackoffBase é a base do backoff exponencial; declarado como var para
// permitir redução em testes (evita sleeps longos).
var retryBackoffBase = 1200 * time.Millisecond

// retryWithBackoff executa fn até obter sucesso ou esgotar as tentativas,
// retentando apenas erros transitórios (isRetryable) com backoff exponencial
// + jitter para evitar rajadas sincronizadas.
func retryWithBackoff(maxRetries int, fn func() error) error {
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isRetryable(err) || attempt == maxRetries {
			return err
		}

		backoff := time.Duration(attempt) * retryBackoffBase
		jitter := time.Duration(rand.Intn(400)) * time.Millisecond
		time.Sleep(backoff + jitter)
	}
	return err
}