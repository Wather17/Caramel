package cli

import (
	"image"
	"image/color"
	"image/png"
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
		{"01_maçã.png", "Maçã"},
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

// writeRealPNG cria um PNG válido em disco (necessário para o gofpdf decodificar)
func writeRealPNG(t *testing.T, path string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 76, G: 175, B: 80, A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("falha ao criar PNG: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("falha ao codificar PNG: %v", err)
	}
}

func TestCardsCmd_FolderGeraPDF(t *testing.T) {
	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "01_maca.png")
	img2 := filepath.Join(tmpDir, "02_banana.png")
	writeRealPNG(t, img1)
	writeRealPNG(t, img2)

	// Reset flags
	cardsCols = 2
	cardsRows = 3
	cardsTitle = "Frutas"
	cardsOutputDir = ""
	cardsUppercase = true
	cardsHTMLMode = false

	err := cardsCmd.RunE(cardsCmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("cardsCmd falhou: %v", err)
	}

	expectedPDF := filepath.Join(tmpDir, filepath.Base(tmpDir)+"_fichas_a4.pdf")
	if _, err := os.Stat(expectedPDF); os.IsNotExist(err) {
		t.Errorf("arquivo PDF esperado não foi criado: %s", expectedPDF)
	}
}

func TestCardsCmd_ModoHTML(t *testing.T) {
	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "01_maca.png")
	writeRealPNG(t, img1)

	cardsCols = 2
	cardsRows = 3
	cardsTitle = "Frutas"
	cardsOutputDir = ""
	cardsUppercase = true
	cardsHTMLMode = true

	err := cardsCmd.RunE(cardsCmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("cardsCmd --html falhou: %v", err)
	}

	expectedHTML := filepath.Join(tmpDir, filepath.Base(tmpDir)+"_fichas_a4.html")
	if _, err := os.Stat(expectedHTML); os.IsNotExist(err) {
		t.Errorf("arquivo HTML esperado não foi criado: %s", expectedHTML)
	}
}

func TestCardsCmd_GradeInvalida(t *testing.T) {
	// Grade fora da faixa deve retornar erro claro
	if err := validateCardGrid(0, 3); err == nil {
		t.Error("esperava erro para cols=0")
	}
	if err := validateCardGrid(2, 7); err == nil {
		t.Error("esperava erro para rows=7")
	}
	if err := validateCardGrid(2, 3); err != nil {
		t.Errorf("grade 2x3 deveria ser válida: %v", err)
	}
}

func TestCardsCmd_InvalidInput(t *testing.T) {
	err := cardsCmd.RunE(cardsCmd, []string{"/caminho/que/nao/existe/12345"})
	if err == nil {
		t.Errorf("esperava erro para caminho inexistente")
	}
}