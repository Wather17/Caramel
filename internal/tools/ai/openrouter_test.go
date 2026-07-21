package ai_test

import (
	"testing"

	"caramel/internal/tools/ai"
)

func TestNewClient(t *testing.T) {
	_, err := ai.NewClient("")
	if err == nil {
		t.Error("Esperado erro ao tentar criar cliente sem APIKey")
	}

	client, err := ai.NewClient("sk-or-v1-teste123")
	if err != nil {
		t.Fatalf("NewClient falhou: %v", err)
	}
	if client == nil {
		t.Error("Cliente criado não deve ser nulo")
	}
}
