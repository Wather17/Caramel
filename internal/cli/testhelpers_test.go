package cli

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// captureStdout redireciona a saída padrão durante a execução de fn e
// retorna tudo o que foi impresso — útil para testar funções que usam fmt.Print*
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("falha ao criar pipe para capturar stdout: %v", err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Errorf("falha ao fechar pipe: %v", err)
	}
	return <-done
}
