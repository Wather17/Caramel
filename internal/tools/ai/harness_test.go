package ai_test

import (
	"testing"
	"time"

	"caramel/internal/tools/ai"
)

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		index    int
		name     string
		expected string
	}{
		{1, "Maçã", "01_ma"},
		{2, "Banana Prata", "02_banana_prata"},
		{10, "Cachorro & Gato", "10_cachorro__gato"},
		{3, "   Espaços   ", "03_espaos"},
		{5, "!!!", "05_item"},
	}

	for _, tt := range tests {
		got := ai.SanitizeSlug(tt.index, tt.name)
		if got != tt.expected {
			t.Errorf("SanitizeSlug(%d, %q) = %q; esperado %q", tt.index, tt.name, got, tt.expected)
		}
	}
}

func TestCalculateConcurrencyDecision(t *testing.T) {
	// 1. Direct Burst (N <= 3)
	workers, delay := ai.CalculateConcurrencyDecision(2, 0)
	if workers != 2 || delay != 0 {
		t.Errorf("esperado (2, 0ms) para N=2, obtido (%d, %v)", workers, delay)
	}

	// 2. Managed Pool (3 < N <= 10)
	workers, delay = ai.CalculateConcurrencyDecision(8, 0)
	if workers != 4 || delay != 150*time.Millisecond {
		t.Errorf("esperado (4, 150ms) para N=8, obtido (%d, %v)", workers, delay)
	}

	// 3. Adaptive Throttle (N > 10)
	workers, delay = ai.CalculateConcurrencyDecision(25, 0)
	if workers != 5 || delay != 300*time.Millisecond {
		t.Errorf("esperado (5, 300ms) para N=25, obtido (%d, %v)", workers, delay)
	}

	// 4. User Override
	workers, delay = ai.CalculateConcurrencyDecision(25, 10)
	if workers != 10 {
		t.Errorf("esperado 10 workers configurados manualmente, obtido %d", workers)
	}
}
