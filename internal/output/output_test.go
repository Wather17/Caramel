package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestOptionsValidateRejeitaQuietComJSON(t *testing.T) {
	if err := (Options{Quiet: true, JSON: true}).Validate(); err == nil {
		t.Fatal("esperava erro ao combinar --quiet e --json")
	}
}

func TestRendererJSONReservaStdoutEEnviaVerboseParaStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	renderer, err := New(Options{JSON: true, Verbose: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("New falhou: %v", err)
	}
	renderer.Progress(Event{Step: "generate", Current: 1, Total: 2, Message: "item"})
	if err := renderer.Result(Result{Status: StateSuccess, Summary: "concluído", Count: 1}); err != nil {
		t.Fatalf("Result falhou: %v", err)
	}
	if !strings.HasPrefix(stdout.String(), `{"status":"success"`) {
		t.Fatalf("stdout deveria conter apenas o resultado JSON, obtido: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "item") {
		t.Fatalf("stderr deveria conter o progresso verbose, obtido: %q", stderr.String())
	}
}

func TestRendererQuietSilenciaResultadoEProgresso(t *testing.T) {
	var stdout, stderr bytes.Buffer
	renderer, err := New(Options{Quiet: true, Verbose: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("New falhou: %v", err)
	}
	renderer.Progress(Event{Message: "não exibir"})
	if err := renderer.Result(Result{Status: StateSuccess, Summary: "não exibir"}); err != nil {
		t.Fatalf("Result falhou: %v", err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("quiet deveria silenciar stdout/stderr, obtido stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRendererQuietMantemErrosNoStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	renderer, err := New(Options{Quiet: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("New falhou: %v", err)
	}
	if err := renderer.Result(Result{Status: StateFailed, Errors: []string{"falha de teste"}}); err != nil {
		t.Fatalf("Result falhou: %v", err)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "falha de teste") {
		t.Fatalf("quiet deveria manter erros em stderr, obtido stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
