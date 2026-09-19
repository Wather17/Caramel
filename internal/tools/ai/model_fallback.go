package ai

import (
	"context"
	"errors"
	"strings"
)

// ModelFallbackEvent descreve a troca do modelo primário para o fallback.
type ModelFallbackEvent struct {
	From  string
	To    string
	Error error
}

// ModelCandidates normaliza uma cadeia e limita a execução a uma troca por
// item. O primeiro fallback não vazio e diferente do primário é o único usado.
func ModelCandidates(primary, defaultModel string, fallbacks []string) []string {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		primary = strings.TrimSpace(defaultModel)
	}
	if primary == "" {
		return nil
	}
	candidates := []string{primary}
	seen := map[string]struct{}{primary: {}}
	for _, fallback := range fallbacks {
		fallback = strings.TrimSpace(fallback)
		if fallback == "" {
			continue
		}
		if _, exists := seen[fallback]; exists {
			continue
		}
		candidates = append(candidates, fallback)
		break
	}
	return candidates
}

// ExecuteModelChainContext executa uma operação com retry por modelo e avança
// para no máximo um fallback após erro transitório esgotado ou erro de contrato.
func ExecuteModelChainContext(ctx context.Context, candidates []string, maxRetries int, call func(model string) error, onFallback func(ModelFallbackEvent)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(candidates) == 0 {
		return "", errors.New("nenhum modelo disponível para a operação")
	}
	if call == nil {
		return candidates[0], errors.New("operação de modelo não configurada")
	}
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for index, model := range candidates {
		if err := ctx.Err(); err != nil {
			return model, err
		}
		err := retryWithBackoffContext(ctx, maxRetries, func() error {
			return call(model)
		})
		if err == nil {
			return model, nil
		}
		if index == len(candidates)-1 || !isFallbackEligible(err) {
			return model, err
		}
		if onFallback != nil {
			onFallback(ModelFallbackEvent{From: model, To: candidates[index+1], Error: err})
		}
	}
	return candidates[len(candidates)-1], context.Canceled
}

func isFallbackEligible(err error) bool {
	if err == nil {
		return false
	}
	if isRetryable(err) {
		return true
	}
	var contractErr *ContractOutputError
	return errors.As(err, &contractErr)
}
