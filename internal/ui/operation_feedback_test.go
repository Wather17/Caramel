package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"caramel/internal/output"
	"caramel/internal/workflow"
)

func TestFormatWorkflowEventUsaEstadosCompartilhados(t *testing.T) {
	cases := []struct {
		name  string
		event workflow.ProgressEvent
		want  string
	}{
		{name: "progresso", event: workflow.ProgressEvent{Step: "colorizing", Current: 1, Total: 2, Message: "gato"}, want: "Em andamento [1/2]: gato"},
		{name: "salvo", event: workflow.ProgressEvent{State: output.StateSuccess, Step: "saved", Message: "gato"}, want: "Salvo: gato"},
		{name: "ignorado", event: workflow.ProgressEvent{State: output.StateSkipped, Step: "skipped", Message: "foto"}, want: "Ignorado: foto"},
		{name: "falha", event: workflow.ProgressEvent{State: output.StateFailed, Step: "error", Message: "gato"}, want: "Falha: gato"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatWorkflowEvent(tc.event); !strings.Contains(got, tc.want) {
				t.Fatalf("formatWorkflowEvent() = %q, esperado trecho %q", got, tc.want)
			}
		})
	}
}

func TestFormatWorkflowCompletionDistingueResultados(t *testing.T) {
	if got := formatWorkflowCompletion(2, nil); !strings.Contains(got, "2 resultado(s)") {
		t.Fatalf("sucesso deveria informar contagem, obtido %q", got)
	}
	if got := formatWorkflowCompletion(0, nil); !strings.Contains(got, "sem resultados") {
		t.Fatalf("resultado vazio deveria ser distinguido, obtido %q", got)
	}
	if got := formatWorkflowCompletion(0, context.Canceled); !strings.Contains(got, "cancelada") {
		t.Fatalf("cancelamento deveria ser distinguido, obtido %q", got)
	}
	if got := formatWorkflowCompletion(0, errors.New("falha")); !strings.Contains(got, "falhou") {
		t.Fatalf("falha deveria ser distinguida, obtido %q", got)
	}
}
