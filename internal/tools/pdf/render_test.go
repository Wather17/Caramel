package pdf

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/phpdave11/gofpdf"
)

func TestRenderPDFPagesRendersCompletePagesAtRequestedDPI(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.pdf")
	writePDFRenderFixture(t, input)

	outputs, err := RenderPDFPages(input, "", "png", 150)
	if err != nil {
		t.Fatalf("RenderPDFPages() falhou: %v", err)
	}
	wantNames := []string{"lesson_page_001.png", "lesson_page_002.png"}
	wantSizes := [][2]int{{1241, 1754}, {1754, 1241}}
	if len(outputs) != len(wantNames) {
		t.Fatalf("RenderPDFPages() gerou %d imagens; esperava %d", len(outputs), len(wantNames))
	}
	for i, output := range outputs {
		if filepath.Base(output) != wantNames[i] {
			t.Errorf("imagem %d chama-se %q; esperava %q", i+1, filepath.Base(output), wantNames[i])
		}
		file, err := os.Open(output)
		if err != nil {
			t.Fatalf("não foi possível abrir imagem %q: %v", output, err)
		}
		rendered, err := png.Decode(file)
		_ = file.Close()
		if err != nil {
			t.Fatalf("não foi possível decodificar imagem %q: %v", output, err)
		}
		bounds := rendered.Bounds()
		if bounds.Dx() != wantSizes[i][0] || bounds.Dy() != wantSizes[i][1] {
			t.Errorf("imagem %d mede %dx%d; esperava %dx%d", i+1, bounds.Dx(), bounds.Dy(), wantSizes[i][0], wantSizes[i][1])
		}
		assertRenderedFixtureContent(t, rendered)
	}
}

func TestRenderPDFPagesSupportsJPEGAndCustomDPI(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.pdf")
	writePDFRenderFixture(t, input)
	outputDir := filepath.Join(dir, "jpeg")

	outputs, err := RenderPDFPages(input, outputDir, "jpeg", 72)
	if err != nil {
		t.Fatalf("RenderPDFPages() com JPEG falhou: %v", err)
	}
	if len(outputs) != 2 || filepath.Base(outputs[0]) != "lesson_page_001.jpg" {
		t.Fatalf("nomes de saída JPEG inesperados: %v", outputs)
	}
	file, err := os.Open(outputs[0])
	if err != nil {
		t.Fatalf("não foi possível abrir JPEG: %v", err)
	}
	config, err := jpeg.DecodeConfig(file)
	_ = file.Close()
	if err != nil {
		t.Fatalf("não foi possível ler JPEG: %v", err)
	}
	if config.Width != 596 || config.Height != 842 {
		t.Errorf("JPEG a 72 DPI mede %dx%d; esperava 596x842", config.Width, config.Height)
	}
}

func TestRenderPDFPagesRejectsUnsafePageBeforeCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "oversized.pdf")
	fixture := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           gofpdf.SizeType{Wd: 5000, Ht: 5000},
	})
	fixture.AddPage()
	if err := fixture.OutputFileAndClose(input); err != nil {
		t.Fatalf("não foi possível criar PDF de dimensões grandes: %v", err)
	}
	outputDir := filepath.Join(dir, "too_large_render")

	if _, err := RenderPDFPages(input, outputDir, "png", 150); err == nil {
		t.Fatal("RenderPDFPages() deveria rejeitar página acima do limite de pixels")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria existir após rejeição de memória; stat err=%v", err)
	}
}

func TestRenderPDFPagesRejectsInvalidInputAndOptionsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.pdf")
	if err := os.WriteFile(invalid, []byte("not a PDF"), 0o600); err != nil {
		t.Fatalf("não foi possível criar PDF inválido: %v", err)
	}
	outputDir := filepath.Join(dir, "invalid_render")
	if _, err := RenderPDFPages(invalid, outputDir, "png", 150); err == nil {
		t.Fatal("RenderPDFPages() deveria rejeitar PDF inválido")
	}
	if _, err := RenderPDFPages(invalid, outputDir, "bmp", 150); err == nil {
		t.Fatal("RenderPDFPages() deveria rejeitar formato não suportado")
	}
	if _, err := RenderPDFPages(invalid, outputDir, "png", 0); err == nil {
		t.Fatal("RenderPDFPages() deveria exigir DPI positivo")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria existir após erros; stat err=%v", err)
	}
}

func TestCommitRenderedPagesRestoresExistingFilesOnPartialPublishFailure(t *testing.T) {
	dir := t.TempDir()
	stageDir := filepath.Join(dir, "stage")
	outputDir := filepath.Join(dir, "output")
	if err := os.Mkdir(stageDir, 0o700); err != nil {
		t.Fatalf("não foi possível criar pasta temporária: %v", err)
	}
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatalf("não foi possível criar pasta de saída: %v", err)
	}

	firstStaged := filepath.Join(stageDir, "page-1.png")
	firstOutput := filepath.Join(outputDir, "page-1.png")
	secondOutput := filepath.Join(outputDir, "page-2.png")
	for path, content := range map[string]string{
		firstStaged:  "new first page",
		firstOutput:  "old first page",
		secondOutput: "old second page",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("não foi possível criar fixture %q: %v", path, err)
		}
	}

	missingStage := filepath.Join(stageDir, "page-2.png")
	err := commitRenderedPages(stageDir, []string{firstStaged, missingStage}, []string{firstOutput, secondOutput}, "")
	if err == nil {
		t.Fatal("commitRenderedPages() deveria falhar quando falta um arquivo preparado")
	}
	for path, want := range map[string]string{
		firstOutput:  "old first page",
		secondOutput: "old second page",
	} {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("não foi possível ler saída restaurada %q: %v", path, readErr)
		}
		if string(got) != want {
			t.Errorf("conteúdo de %q = %q; esperava preservar %q", path, got, want)
		}
	}
}

func writePDFRenderFixture(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	imagePath := filepath.Join(dir, "render-fixture.png")
	f, err := os.Create(imagePath)
	if err != nil {
		t.Fatalf("não foi possível criar imagem da fixture: %v", err)
	}
	fixtureImage := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			fixtureImage.Set(x, y, color.RGBA{R: 240, G: 30, B: 20, A: 255})
		}
	}
	if err := png.Encode(f, fixtureImage); err != nil {
		_ = f.Close()
		t.Fatalf("não foi possível codificar imagem da fixture: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("não foi possível fechar imagem da fixture: %v", err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	for _, orientation := range []string{"P", "L"} {
		pdf.AddPageFormat(orientation, gofpdf.SizeType{Wd: 210, Ht: 297})
		pdf.SetFont("Helvetica", "", 18)
		pdf.Text(20, 40, "Text and vectors")
		pdf.SetFillColor(20, 40, 220)
		pdf.Rect(90, 80, 20, 15, "F")
		pdf.ImageOptions(imagePath, 50, 100, 20, 20, false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
	}
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatalf("não foi possível criar PDF de fixture: %v", err)
	}
}

func assertRenderedFixtureContent(t *testing.T, rendered image.Image) {
	t.Helper()
	assertPixelNear(t, rendered, 100, 87, 150, color.RGBA{R: 20, G: 40, B: 220, A: 255})
	assertPixelNear(t, rendered, 60, 110, 150, color.RGBA{R: 240, G: 30, B: 20, A: 255})
	blackPixels := 0
	bounds := rendered.Bounds()
	for y := 170; y < int(math.Min(float64(bounds.Max.Y), 280)); y++ {
		for x := 115; x < int(math.Min(float64(bounds.Max.X), 390)); x++ {
			r, g, b, _ := rendered.At(x, y).RGBA()
			if r < 0x5000 && g < 0x5000 && b < 0x5000 {
				blackPixels++
			}
		}
	}
	if blackPixels == 0 {
		t.Error("renderização deveria conter o texto da página")
	}
}

func assertPixelNear(t *testing.T, rendered image.Image, xMM, yMM float64, dpi int, want color.RGBA) {
	t.Helper()
	x := int(math.Round(xMM * float64(dpi) / 25.4))
	y := int(math.Round(yMM * float64(dpi) / 25.4))
	got := color.RGBAModel.Convert(rendered.At(x, y)).(color.RGBA)
	if got.R != want.R || got.G != want.G || got.B != want.B {
		t.Errorf("pixel em %.0fmm, %.0fmm = %v; esperava %v", xMM, yMM, got, want)
	}
}
