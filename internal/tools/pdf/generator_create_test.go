package pdf

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestGenerateImagePDFPreservesOrderAndChoosesA4Orientation(t *testing.T) {
	dir := t.TempDir()
	portrait := filepath.Join(dir, "portrait.png")
	landscape := filepath.Join(dir, "landscape.png")
	square := filepath.Join(dir, "square.png")
	writeCreateImageFixture(t, portrait, 30, 45)
	writeCreateImageFixture(t, landscape, 60, 40)
	writeCreateImageFixture(t, square, 40, 40)
	output := filepath.Join(dir, "images.pdf")

	count, err := GenerateImagePDF([]string{portrait, landscape, square}, output, DefaultOptions())
	if err != nil {
		t.Fatalf("GenerateImagePDF() falhou: %v", err)
	}
	if count != 3 {
		t.Fatalf("GenerateImagePDF() retornou %d páginas; esperava 3", count)
	}

	dims, err := api.PageDimsFile(output)
	if err != nil {
		t.Fatalf("não foi possível ler dimensões do PDF: %v", err)
	}
	if len(dims) != 3 {
		t.Fatalf("PDF contém %d páginas; esperava 3", len(dims))
	}
	wantLandscape := []bool{false, true, false}
	for i, dimension := range dims {
		gotLandscape := dimension.Width > dimension.Height
		if gotLandscape != wantLandscape[i] {
			t.Errorf("página %d tem orientação landscape=%t; esperava %t", i+1, gotLandscape, wantLandscape[i])
		}
		wantWidth, wantHeight := 210.0*72/25.4, 297.0*72/25.4
		if wantLandscape[i] {
			wantWidth, wantHeight = wantHeight, wantWidth
		}
		if !closeEnough(dimension.Width, wantWidth) || !closeEnough(dimension.Height, wantHeight) {
			t.Errorf("página %d mede %.2fx%.2f pt; esperava A4 %.2fx%.2f pt", i+1, dimension.Width, dimension.Height, wantWidth, wantHeight)
		}
	}
}

func TestGenerateImagePDFDoesNotLeaveOutputForInvalidImage(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.png")
	invalid := filepath.Join(dir, "invalid.png")
	writeCreateImageFixture(t, valid, 30, 45)
	if err := os.WriteFile(invalid, []byte("not an image"), 0o600); err != nil {
		t.Fatalf("não foi possível criar imagem inválida: %v", err)
	}
	output := filepath.Join(dir, "partial.pdf")

	if _, err := GenerateImagePDF([]string{valid, invalid}, output, DefaultOptions()); err == nil {
		t.Fatal("GenerateImagePDF() deveria rejeitar imagem inválida")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("PDF parcial não deveria existir; stat err=%v", err)
	}
}

func TestGenerateImagePDFRejectsOutputThatAliasesInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "image.png")
	writeCreateImageFixture(t, input, 30, 45)
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("não foi possível ler imagem de entrada: %v", err)
	}

	if _, err := GenerateImagePDF([]string{input}, input, DefaultOptions()); err == nil {
		t.Fatal("GenerateImagePDF() deveria rejeitar saída igual à entrada")
	}
	after, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("não foi possível reler imagem de entrada: %v", err)
	}
	if string(after) != string(original) {
		t.Fatal("GenerateImagePDF() alterou a imagem de entrada")
	}
}

func TestGenerateImagePDFRequiresImagesAndOutput(t *testing.T) {
	if _, err := GenerateImagePDF(nil, "out.pdf", DefaultOptions()); err == nil {
		t.Fatal("GenerateImagePDF() deveria exigir ao menos uma imagem")
	}
	if _, err := GenerateImagePDF([]string{"image.png"}, " ", DefaultOptions()); err == nil {
		t.Fatal("GenerateImagePDF() deveria exigir caminho de saída")
	}
}

func TestContainFitPreservesImageAspectRatio(t *testing.T) {
	w, h := calculateFitDimensions(600, 400, 200, 287, "contain")
	if !closeEnough(w/h, 1.5) {
		t.Fatalf("proporção da imagem foi alterada: %.4f; esperava 1.5", w/h)
	}
	if w > 200 || h > 287 {
		t.Fatalf("imagem excedeu a área A4 útil: %.2fx%.2f", w, h)
	}
}

func writeCreateImageFixture(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 50, G: 120, B: 200, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("não foi possível criar fixture %q: %v", path, err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("não foi possível codificar fixture %q: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("não foi possível fechar fixture %q: %v", path, err)
	}
}
