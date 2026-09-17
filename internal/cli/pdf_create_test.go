package cli

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestPDFCreateCommandUsesDefaultFileName(t *testing.T) {
	configurePDFCreateTestFlags()
	dir := t.TempDir()
	input := filepath.Join(dir, "worksheet.png")
	writePDFCreateImage(t, input, 24, 36)
	output := filepath.Join(dir, "worksheet.pdf")

	stdout, err := runPDFCreateForTest(t, input)
	if err != nil {
		t.Fatalf("pdf create falhou: %v", err)
	}
	assertPDFCreateResult(t, stdout, output, 1)
}

func TestPDFCreateCommandUsesNaturalFolderOrderAndName(t *testing.T) {
	configurePDFCreateTestFlags()
	dir := t.TempDir()
	folder := filepath.Join(dir, "activity_images")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatalf("não foi possível criar pasta de imagens: %v", err)
	}
	writePDFCreateImage(t, filepath.Join(folder, "page1.png"), 24, 36)
	writePDFCreateImage(t, filepath.Join(folder, "page2.png"), 48, 24)
	writePDFCreateImage(t, filepath.Join(folder, "page10.png"), 24, 36)
	if err := os.WriteFile(filepath.Join(folder, "ignore.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatalf("não foi possível criar arquivo de texto: %v", err)
	}
	output := filepath.Join(dir, "activity_images.pdf")

	stdout, err := runPDFCreateForTest(t, folder)
	if err != nil {
		t.Fatalf("pdf create da pasta falhou: %v", err)
	}
	assertPDFCreateResult(t, stdout, output, 3)
	assertPDFOrientationOrder(t, output, []bool{false, true, false})
}

func TestPDFCreateCommandPreservesExplicitOrderAndOutput(t *testing.T) {
	configurePDFCreateTestFlags()
	dir := t.TempDir()
	landscape := filepath.Join(dir, "landscape.png")
	portrait := filepath.Join(dir, "portrait.png")
	output := filepath.Join(dir, "custom", "collection.pdf")
	writePDFCreateImage(t, landscape, 48, 24)
	writePDFCreateImage(t, portrait, 24, 36)
	if err := pdfCreateCmd.Flags().Set("output", output); err != nil {
		t.Fatalf("não foi possível configurar --output: %v", err)
	}

	stdout, err := runPDFCreateForTest(t, landscape, portrait)
	if err != nil {
		t.Fatalf("pdf create com arquivos explícitos falhou: %v", err)
	}
	assertPDFCreateResult(t, stdout, output, 2)
	assertPDFOrientationOrder(t, output, []bool{true, false})
}

func TestPDFCreateCommandUsesCollectionDefaultName(t *testing.T) {
	configurePDFCreateTestFlags()
	dir := t.TempDir()
	first := filepath.Join(dir, "cover.png")
	second := filepath.Join(dir, "activity.png")
	writePDFCreateImage(t, first, 48, 24)
	writePDFCreateImage(t, second, 24, 36)
	output := filepath.Join(dir, "cover_collection.pdf")

	stdout, err := runPDFCreateForTest(t, first, second)
	if err != nil {
		t.Fatalf("pdf create com lista de imagens falhou: %v", err)
	}
	assertPDFCreateResult(t, stdout, output, 2)
}

func TestPDFCreateCommandRejectsInvalidImageWithoutPartialPDF(t *testing.T) {
	configurePDFCreateTestFlags()
	dir := t.TempDir()
	input := filepath.Join(dir, "invalid.png")
	if err := os.WriteFile(input, []byte("not an image"), 0o600); err != nil {
		t.Fatalf("não foi possível criar fixture inválida: %v", err)
	}
	output := filepath.Join(dir, "invalid.pdf")

	if _, err := runPDFCreateForTest(t, input); err == nil {
		t.Fatal("pdf create deveria rejeitar imagem inválida")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("PDF parcial não deveria existir; stat err=%v", err)
	}
}

func configurePDFCreateTestFlags() {
	pdfCreateOutput = ""
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}

func runPDFCreateForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	oldOut := pdfCreateCmd.OutOrStdout()
	var stdout bytes.Buffer
	pdfCreateCmd.SetOut(&stdout)
	defer pdfCreateCmd.SetOut(oldOut)
	err := pdfCreateCmd.RunE(pdfCreateCmd, args)
	return stdout.String(), err
}

func assertPDFCreateResult(t *testing.T, stdout, output string, pages int) {
	t.Helper()
	if !strings.Contains(stdout, output) {
		t.Errorf("resultado não informa saída %q: %s", output, stdout)
	}
	if !strings.Contains(stdout, "página(s)") {
		t.Errorf("resultado não resume a contagem de páginas: %s", stdout)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("PDF de saída não foi criado: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF de saída está vazio")
	}
	dims, err := api.PageDimsFile(output)
	if err != nil {
		t.Fatalf("não foi possível ler páginas do PDF: %v", err)
	}
	if len(dims) != pages {
		t.Fatalf("PDF contém %d páginas; esperava %d", len(dims), pages)
	}
}

func assertPDFOrientationOrder(t *testing.T, output string, wantLandscape []bool) {
	t.Helper()
	dims, err := api.PageDimsFile(output)
	if err != nil {
		t.Fatalf("não foi possível ler orientações do PDF: %v", err)
	}
	for i, dimension := range dims {
		gotLandscape := dimension.Width > dimension.Height
		if gotLandscape != wantLandscape[i] {
			t.Errorf("página %d tem orientação landscape=%t; esperava %t", i+1, gotLandscape, wantLandscape[i])
		}
	}
}

func writePDFCreateImage(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 90, B: 40, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("não foi possível criar imagem %q: %v", path, err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("não foi possível codificar imagem %q: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("não foi possível fechar imagem %q: %v", path, err)
	}
}
