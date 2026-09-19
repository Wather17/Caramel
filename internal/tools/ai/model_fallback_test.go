package ai_test

import (
	"context"
	"errors"
	"testing"

	"caramel/internal/tools/ai"
)

func TestModelCandidatesLimitaUmaTrocaEDeduplica(t *testing.T) {
	got := ai.ModelCandidates("primary", "default", []string{"primary", "fallback-1", "fallback-2"})
	if len(got) != 2 || got[0] != "primary" || got[1] != "fallback-1" {
		t.Fatalf("cadeia inesperada: %#v", got)
	}
}

func TestExecuteModelChainUsaFallbackEmErroDeContrato(t *testing.T) {
	var calls []string
	var transitions []ai.ModelFallbackEvent
	contractErr := ai.NewContractOutputError("síntese", "primary", ai.StructuredJSONArray, "texto", "resposta inválida")
	model, err := ai.ExecuteModelChainContext(context.Background(), []string{"primary", "fallback"}, 3, func(model string) error {
		calls = append(calls, model)
		if model == "primary" {
			return contractErr
		}
		return nil
	}, func(event ai.ModelFallbackEvent) {
		transitions = append(transitions, event)
	})
	if err != nil || model != "fallback" {
		t.Fatalf("fallback deveria concluir com fallback: model=%s err=%v", model, err)
	}
	if len(calls) != 2 || calls[0] != "primary" || calls[1] != "fallback" || len(transitions) != 1 {
		t.Fatalf("execução inesperada: calls=%v transitions=%v", calls, transitions)
	}
}

func TestExecuteModelChainNaoFazFallbackParaErroPermanente(t *testing.T) {
	calls := 0
	permanent := errors.New("401 sem autorização")
	model, err := ai.ExecuteModelChainContext(context.Background(), []string{"primary", "fallback"}, 3, func(model string) error {
		calls++
		return permanent
	}, nil)
	if !errors.Is(err, permanent) || model != "primary" || calls != 1 {
		t.Fatalf("erro permanente deveria encerrar no primário: model=%s calls=%d err=%v", model, calls, err)
	}
}

func TestExecuteModelChainNaoFazFallbackDepoisDeCancelamento(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	model, err := ai.ExecuteModelChainContext(ctx, []string{"primary", "fallback"}, 3, func(model string) error {
		calls++
		return errors.New("não deveria ser chamado")
	}, nil)
	if !errors.Is(err, context.Canceled) || model != "primary" || calls != 0 {
		t.Fatalf("cancelamento deveria encerrar sem fallback: model=%s calls=%d err=%v", model, calls, err)
	}
}

func TestExecuteModelChainRejeitaCadeiaVazia(t *testing.T) {
	model, err := ai.ExecuteModelChainContext(context.Background(), nil, 3, func(model string) error {
		return nil
	}, nil)
	if model != "" || err == nil {
		t.Fatalf("cadeia vazia deveria falhar sem modelo: model=%q err=%v", model, err)
	}
}
