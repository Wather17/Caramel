package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanCardName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"01_maca.png", "Maca"},
		{"02_banana_prata.jpg", "Banana Prata"},
		{"10-cachorro-quente.webp", "Cachorro Quente"},
		{"morango.png", "Morango"},
	}

	for _, tt := range tests {
		got := CleanCardName(tt.input)
		if got != tt.expected {
			t.Errorf("CleanCardName(%q) = %q; esperado %q", tt.input, got, tt.expected)
		}
	}
}

func TestCardsCmd_Folder(t *testing.T) {
	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "01_maca.png")
	img2 := filepath.Join(tmpDir, "02_banana.png")
	_ = os.WriteFile(img1, []byte("fake-bytes"), 0644)
	_ = os.WriteFile(img2, []byte("fake-bytes"), 0644)

	// Reset flags
	cardsCols = 2
	cardsRows = 3
	cardsTitle = "Frutas"
	cardsOutputDir = ""
	cardsCutLines = true
	cardsUppercase = true
	cardsEmbedB64 = true

	err := cardsCmd.RunE(cardsCmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("cardsCmd falhou: %v", err)
	}

	expectedHTML := filepath.Join(tmpDir, filepath.Base(tmpDir)+"_fichas_a4.html")
	if _, err := os.Stat(expectedHTML); os.IsNotExist(err) {
		t.Errorf("arquivo HTML esperado não foi criado: %s", expectedHTML)
	}
}

func TestCardsCmd_InvalidInput(t *testing.T) {
	err := cardsCmd.RunE(cardsCmd, []string{"/caminho/que/nao/existe/12345"})
	if err == nil {
		t.Errorf("esperava erro para caminho inexistente")
	}
}
