package ui

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderImageToANSI_NilOrEmpty(t *testing.T) {
	output := RenderImageToANSI(nil, 20, 10)
	if output != "" {
		t.Errorf("esperava string vazia para imagem nil, obteve %q", output)
	}

	emptyImg := image.NewRGBA(image.Rect(0, 0, 0, 0))
	outputEmpty := RenderImageToANSI(emptyImg, 20, 10)
	if outputEmpty != "" {
		t.Errorf("esperava string vazia para imagem sem dimensões, obteve %q", outputEmpty)
	}

	validImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if outputZeroCols := RenderImageToANSI(validImg, 0, 10); outputZeroCols != "" {
		t.Errorf("esperava string vazia para maxCols=0, obteve %q", outputZeroCols)
	}
}

func TestRenderImageToANSI_ValidImage(t *testing.T) {
	// Cria uma imagem simples 4x4
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}

	draw.Draw(img, image.Rect(0, 0, 4, 2), &image.Uniform{C: red}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 2, 4, 4), &image.Uniform{C: blue}, image.Point{}, draw.Src)

	output := RenderImageToANSI(img, 10, 5)
	if output == "" {
		t.Fatal("esperava saída ANSI não vazia para imagem válida")
	}

	if !strings.Contains(output, "▀") {
		t.Errorf("esperava caractere half-block '▀' na saída ANSI")
	}

	if !strings.Contains(output, "\x1b[38;2;") {
		t.Errorf("esperava sequências de cor ANSI TrueColor de primeiro plano '\\x1b[38;2;'")
	}

	if !strings.Contains(output, "\x1b[48;2;") {
		t.Errorf("esperava sequências de cor ANSI TrueColor de fundo '\\x1b[48;2;'")
	}
}

func TestRenderImageFileToANSI(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: green}, image.Point{}, draw.Src)

	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("falha ao criar arquivo de teste: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("falha ao codificar PNG de teste: %v", err)
	}
	f.Close()

	ansiStr, err := RenderImageFileToANSI(imgPath, 10, 5)
	if err != nil {
		t.Fatalf("erro inesperado ao renderizar arquivo de imagem: %v", err)
	}

	if ansiStr == "" {
		t.Errorf("esperava string ANSI não vazia do arquivo")
	}

	// Teste com arquivo inexistente
	_, errMissing := RenderImageFileToANSI(filepath.Join(tmpDir, "non_existent.png"), 10, 5)
	if errMissing == nil {
		t.Errorf("esperava erro ao tentar abrir arquivo inexistente")
	}
}
