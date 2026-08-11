package pdf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClean2UpSuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1º DOMINGO (02.08.26) – PROF.ª Ieda", "1º DOMINGO (02.08.26) – PROF.ª Ieda"},
		{"1º DOMINGO (02.08.26) – PROF.ª Ieda_2up", "1º DOMINGO (02.08.26) – PROF.ª Ieda"},
		{"atividade_2up_2up", "atividade"},
		{"teste_2UP", "teste"},
		{"  minha_atividade_2up  ", "minha_atividade"},
	}

	for _, tt := range tests {
		result := Clean2UpSuffix(tt.input)
		if result != tt.expected {
			t.Errorf("Clean2UpSuffix(%q) = %q; esperava %q", tt.input, result, tt.expected)
		}
	}
}

func TestResolveFuzzyPath(t *testing.T) {
	tempDir := t.TempDir()

	// Cria um arquivo com caracteres especiais e en-dash no disco
	realFileName := "1º DOMINGO (02.08.26) – PROF.ª Ieda.png"
	realPath := filepath.Join(tempDir, realFileName)

	err := os.WriteFile(realPath, []byte("fake image content"), 0644)
	if err != nil {
		t.Fatalf("Falha ao criar arquivo de teste: %v", err)
	}

	t.Run("Exact Match", func(t *testing.T) {
		resolved, stat, err := ResolveFuzzyPath(realPath)
		if err != nil {
			t.Fatalf("Resolução exata falhou: %v", err)
		}
		if resolved != realPath {
			t.Errorf("Esperava %s, obteve %s", realPath, resolved)
		}
		if stat == nil {
			t.Errorf("FileInfo retornou nil")
		}
	})

	t.Run("Fuzzy Match with Regular Hyphen and Different Case", func(t *testing.T) {
		// Caminho fornecido com hífen ASCII '-' em vez de en-dash '–'
		fuzzyInput := filepath.Join(tempDir, "1º DOMINGO (02.08.26) - PROF.ª Ieda.png")
		resolved, stat, err := ResolveFuzzyPath(fuzzyInput)
		if err != nil {
			t.Fatalf("Resolução fuzzy falhou: %v", err)
		}
		if resolved != realPath {
			t.Errorf("Fuzzy resolve esperava %s, obteve %s", realPath, resolved)
		}
		if stat == nil {
			t.Errorf("FileInfo retornou nil")
		}
	})
}
