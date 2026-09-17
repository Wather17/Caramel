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

	"github.com/phpdave11/gofpdf"
)

func TestPDFImagesExtractCommandUsesOutputDirectory(t *testing.T) {
	configurePDFImagesTestFlags(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.pdf")
	writePDFImagesExtractFixture(t, input)
	outputDir := filepath.Join(dir, "extracted")
	if err := pdfImagesExtractCmd.Flags().Set("output-dir", outputDir); err != nil {
		t.Fatalf("não foi possível configurar --output-dir: %v", err)
	}

	stdout, err := runPDFImagesExtractForTest(t, input)
	if err != nil {
		t.Fatalf("pdf images extract falhou: %v", err)
	}
	output := filepath.Join(outputDir, "lesson_page_001_image_001.png")
	if !strings.Contains(stdout, output) || !strings.Contains(stdout, "1 arquivo(s)") {
		t.Errorf("resultado da CLI não informa saída e quantidade: %q", stdout)
	}
	if _, err := os.Stat(output); err != nil {
		t.Errorf("imagem extraída não foi criada em %q: %v", output, err)
	}
}

func TestPDFImagesExtractCommandReportsNoImagesAsWarning(t *testing.T) {
	configurePDFImagesTestFlags(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "text-only.pdf")
	writePDFSplitFixture(t, input, 1)

	stdout, err := runPDFImagesExtractForTest(t, input)
	if err != nil {
		t.Fatalf("PDF sem imagens deveria retornar aviso em vez de erro: %v", err)
	}
	if !strings.Contains(stdout, "Nenhuma imagem embutida foi extraída") {
		t.Errorf("resumo da CLI não explica que não havia imagens: %q", stdout)
	}
}

func configurePDFImagesTestFlags(t *testing.T) {
	t.Helper()
	pdfImagesOutputDir = ""
	pdfImagesExtractCmd.Flags().Lookup("output-dir").Changed = false
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}

func runPDFImagesExtractForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	oldOut := pdfImagesExtractCmd.OutOrStdout()
	var stdout bytes.Buffer
	pdfImagesExtractCmd.SetOut(&stdout)
	defer pdfImagesExtractCmd.SetOut(oldOut)
	err := pdfImagesExtractCmd.RunE(pdfImagesExtractCmd, args)
	return stdout.String(), err
}

func writePDFImagesExtractFixture(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	imagePath := filepath.Join(dir, "embedded-image.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 120, B: 210, A: 255})
		}
	}
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatalf("não foi possível criar imagem fixture: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("não foi possível codificar imagem fixture: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("não foi possível fechar imagem fixture: %v", err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.ImageOptions(imagePath, 20, 20, 25, 25, false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatalf("não foi possível criar PDF fixture: %v", err)
	}
}
