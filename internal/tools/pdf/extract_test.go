package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"
)

func TestExtractEmbeddedImagesPreservesFormatsAcrossPages(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.pdf")
	writePDFExtractFixture(t, input)

	paths, warnings, err := ExtractEmbeddedImages(input, "")
	if err != nil {
		t.Fatalf("ExtractEmbeddedImages() falhou: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("extração completa retornou avisos inesperados: %v", warnings)
	}
	if len(paths) != 3 {
		t.Fatalf("ExtractEmbeddedImages() gerou %d arquivos; esperava 3: %v", len(paths), paths)
	}
	allowedNames := map[string]bool{
		"lesson_page_001_image_001.png": true,
		"lesson_page_001_image_001.jpg": true,
		"lesson_page_001_image_002.png": true,
		"lesson_page_001_image_002.jpg": true,
		"lesson_page_003_image_001.png": true,
	}
	pageOneExtensions := make(map[string]bool)
	for _, path := range paths {
		name := filepath.Base(path)
		if !allowedNames[name] {
			t.Errorf("nome determinístico inesperado para imagem extraída: %q", name)
		}
		delete(allowedNames, name)
		if strings.HasPrefix(name, "lesson_page_001_") {
			pageOneExtensions[filepath.Ext(name)] = true
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("não foi possível abrir imagem %q: %v", path, err)
		}
		switch filepath.Ext(path) {
		case ".png":
			if _, err := png.DecodeConfig(file); err != nil {
				t.Errorf("arquivo PNG %q não corresponde a PNG: %v", path, err)
			}
		case ".jpg":
			if _, err := jpeg.DecodeConfig(file); err != nil {
				t.Errorf("arquivo JPG %q não corresponde a JPEG: %v", path, err)
			}
		default:
			t.Errorf("extensão inesperada na saída %q", path)
		}
		_ = file.Close()
	}
	if len(pageOneExtensions) != 2 || !pageOneExtensions[".png"] || !pageOneExtensions[".jpg"] {
		t.Errorf("a página 1 deveria produzir uma imagem PNG e uma JPEG: %v", paths)
	}
}

func TestExtractEmbeddedImagesReportsPDFWithoutImages(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "text-only.pdf")
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 14)
	pdf.Text(20, 30, "Text only")
	if err := pdf.OutputFileAndClose(input); err != nil {
		t.Fatalf("não foi possível criar PDF sem imagens: %v", err)
	}
	outputDir := filepath.Join(dir, "text-only_embedded_images")

	paths, warnings, err := ExtractEmbeddedImages(input, "")
	if err != nil {
		t.Fatalf("PDF sem imagens deveria retornar um resultado claro: %v", err)
	}
	if len(paths) != 0 || len(warnings) != 0 {
		t.Fatalf("resultado inesperado para PDF sem imagens: paths=%v warnings=%v", paths, warnings)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Errorf("pasta de saída não deveria ser criada sem imagens; stat err=%v", err)
	}
}

func TestExtractEmbeddedImagesDoesNotOverwriteExistingFiles(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.pdf")
	writePDFExtractFixture(t, input)
	outputDir := filepath.Join(dir, "existing")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatalf("não foi possível criar a pasta de saída: %v", err)
	}
	firstOutput := filepath.Join(outputDir, "lesson_page_001_image_001.png")
	if err := os.WriteFile(firstOutput, []byte("user data"), 0o600); err != nil {
		t.Fatalf("não foi possível criar arquivo existente: %v", err)
	}

	if _, _, err := ExtractEmbeddedImages(input, outputDir); err == nil || !strings.Contains(err.Error(), "já existe") {
		t.Fatalf("esperava rejeição explícita de saída existente, recebeu %v", err)
	}
	got, err := os.ReadFile(firstOutput)
	if err != nil {
		t.Fatalf("arquivo existente foi removido: %v", err)
	}
	if string(got) != "user data" {
		t.Fatalf("arquivo existente foi alterado: %q", got)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("não foi possível listar pasta de saída: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("a extração deixou saídas parciais: %v", entries)
	}
}

func TestExtractEmbeddedImagesRejectsInvalidPDFWithoutCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "invalid.pdf")
	if err := os.WriteFile(input, []byte("not a PDF"), 0o600); err != nil {
		t.Fatalf("não foi possível criar entrada inválida: %v", err)
	}
	outputDir := filepath.Join(dir, "images")

	if _, _, err := ExtractEmbeddedImages(input, outputDir); err == nil {
		t.Fatal("ExtractEmbeddedImages() deveria rejeitar PDF inválido")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Errorf("pasta de saída não deveria existir após erro: %v", err)
	}
}

func TestExtractEmbeddedImagesWarnsForUnsupportedResource(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "unsupported.pdf")
	writePDFWithUnsupportedImage(t, input)
	outputDir := filepath.Join(dir, "unsupported_embedded_images")

	paths, warnings, err := ExtractEmbeddedImages(input, outputDir)
	if err != nil {
		t.Fatalf("recurso não suportado deveria gerar aviso: %v", err)
	}
	if len(paths) != 1 || len(warnings) == 0 {
		t.Fatalf("recurso não suportado deve gerar aviso sem descartar imagens válidas: paths=%v warnings=%v", paths, warnings)
	}
	if filepath.Base(paths[0]) != "unsupported_page_001_image_001.png" {
		t.Errorf("nome da imagem válida inesperado: %q", paths[0])
	}
	file, err := os.Open(paths[0])
	if err != nil {
		t.Fatalf("imagem válida da página 1 não foi publicada: %v", err)
	}
	if _, err := png.DecodeConfig(file); err != nil {
		t.Errorf("arquivo publicado não corresponde à extensão PNG: %v", err)
	}
	_ = file.Close()
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("não foi possível listar saídas parciais bem-sucedidas: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("extração criou saídas inesperadas: %v", entries)
	}
}

func TestPublishExtractedImagesRollsBackPartialPublication(t *testing.T) {
	dir := t.TempDir()
	stageDir := filepath.Join(dir, "stage")
	outputDir := filepath.Join(dir, "new-output")
	if err := os.Mkdir(stageDir, 0o700); err != nil {
		t.Fatalf("não foi possível criar pasta de estágio: %v", err)
	}
	firstStaged := filepath.Join(stageDir, "image-1.png")
	if err := os.WriteFile(firstStaged, []byte("image data"), 0o600); err != nil {
		t.Fatalf("não foi possível criar imagem preparada: %v", err)
	}

	err := publishExtractedImages(
		[]string{firstStaged, filepath.Join(stageDir, "missing.png")},
		[]string{filepath.Join(outputDir, "image-1.png"), filepath.Join(outputDir, "image-2.png")},
	)
	if err == nil {
		t.Fatal("publicação deveria falhar se faltar uma imagem preparada")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Errorf("rollback deveria remover a pasta criada; stat err=%v", err)
	}
}

func writePDFExtractFixture(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	pngPath := filepath.Join(dir, "embedded.png")
	jpgPath := filepath.Join(dir, "embedded.jpg")
	fixtureImage := image.NewRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < fixtureImage.Bounds().Dy(); y++ {
		for x := 0; x < fixtureImage.Bounds().Dx(); x++ {
			fixtureImage.Set(x, y, color.RGBA{R: 40, G: 180, B: 80, A: 255})
		}
	}
	writeImage := func(path string, encode func(*bytes.Buffer, image.Image) error) {
		t.Helper()
		var data bytes.Buffer
		if err := encode(&data, fixtureImage); err != nil {
			t.Fatalf("não foi possível codificar fixture de imagem: %v", err)
		}
		if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
			t.Fatalf("não foi possível salvar fixture %q: %v", path, err)
		}
	}
	writeImage(pngPath, func(w *bytes.Buffer, img image.Image) error { return png.Encode(w, img) })
	writeImage(jpgPath, func(w *bytes.Buffer, img image.Image) error { return jpeg.Encode(w, img, &jpeg.Options{Quality: 90}) })

	pdf := gofpdf.New("P", "mm", "A4", "")
	for page := 1; page <= 3; page++ {
		pdf.AddPage()
		if page == 1 {
			pdf.ImageOptions(pngPath, 20, 20, 25, 25, false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
			pdf.ImageOptions(jpgPath, 60, 20, 25, 25, false, gofpdf.ImageOptions{ImageType: "JPG", ReadDpi: true}, 0, "")
		}
		if page == 3 {
			pdf.ImageOptions(pngPath, 20, 20, 25, 25, false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
		}
	}
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatalf("não foi possível criar fixture PDF: %v", err)
	}
}

func writePDFWithUnsupportedImage(t *testing.T, path string) {
	t.Helper()
	content := []byte("q 50 0 0 50 0 0 cm /Im0 Do Q")
	unsupportedContent := []byte("q 50 0 0 50 0 0 cm /Im1 Do Q")
	var compressedImage bytes.Buffer
	imageWriter := zlib.NewWriter(&compressedImage)
	if _, err := imageWriter.Write([]byte{40, 180, 80}); err != nil {
		t.Fatalf("não foi possível compactar fixture de imagem: %v", err)
	}
	if err := imageWriter.Close(); err != nil {
		t.Fatalf("não foi possível fechar fixture de imagem compactada: %v", err)
	}
	imageData := []byte("unsupported image data")
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(content), content),
		fmt.Sprintf("5 0 obj\n<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream\nendobj", compressedImage.Len(), compressedImage.Bytes()),
		"6 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /XObject << /Im1 8 0 R >> >> /Contents 7 0 R >>\nendobj",
		fmt.Sprintf("7 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(unsupportedContent), unsupportedContent),
		fmt.Sprintf("8 0 obj\n<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /UnknownDecode /Length %d >>\nstream\n%s\nendstream\nendobj", len(imageData), imageData),
	}
	var data bytes.Buffer
	data.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = data.Len()
		data.WriteString(object)
		data.WriteByte('\n')
	}
	xrefOffset := data.Len()
	fmt.Fprintf(&data, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&data, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&data, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatalf("não foi possível criar PDF com filtro desconhecido: %v", err)
	}
}
