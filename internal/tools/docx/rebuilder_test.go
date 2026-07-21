package docx_test

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"caramel/internal/tools/docx"
)

// createDummyPNG cria uma imagem PNG simples em memória
func createDummyPNG(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	drawColor := color.RGBA{0, 0, 255, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, drawColor)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("Erro ao encodar PNG de teste: %v", err)
	}
	return buf.Bytes()
}

func TestResizeToMatch(t *testing.T) {
	tempDir := t.TempDir()

	origPath := filepath.Join(tempDir, "orig.png")
	colorizedPath := filepath.Join(tempDir, "colorized.png")

	// Cria imagem original (100x100) e colorida (300x200)
	origBytes := createDummyPNG(t, 100, 100)
	colorizedBytes := createDummyPNG(t, 300, 200)

	os.WriteFile(origPath, origBytes, 0644)
	os.WriteFile(colorizedPath, colorizedBytes, 0644)

	resizedBytes, err := docx.ResizeToMatch(origPath, colorizedPath)
	if err != nil {
		t.Fatalf("ResizeToMatch falhou: %v", err)
	}

	// Decodifica imagem de saída e checa dimensões
	img, _, err := image.Decode(bytes.NewReader(resizedBytes))
	if err != nil {
		t.Fatalf("Falha ao decodificar imagem redimensionada: %v", err)
	}

	if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 100 {
		t.Errorf("Esperado tamanho 100x100, obtido %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestRebuildDocx(t *testing.T) {
	tempDir := t.TempDir()

	origZipPath := filepath.Join(tempDir, "orig.docx")
	destZipPath := filepath.Join(tempDir, "dest.docx")

	// 1. Cria zip de origem
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("word/media/image1.png")
	w.Write([]byte("original image bytes"))
	w2, _ := zw.Create("document.xml")
	w2.Write([]byte("xml content"))
	zw.Close()
	os.WriteFile(origZipPath, buf.Bytes(), 0644)

	// 2. Tenta reconstruir zip substituindo a imagem
	replacements := map[string][]byte{
		"word/media/image1.png": []byte("new replaced image bytes"),
	}

	err := docx.RebuildDocx(origZipPath, destZipPath, replacements)
	if err != nil {
		t.Fatalf("RebuildDocx falhou: %v", err)
	}

	// 3. Lê zip de destino e checa se substituição funcionou
	r, err := zip.OpenReader(destZipPath)
	if err != nil {
		t.Fatalf("Falha ao abrir zip de destino: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()

		if f.Name == "word/media/image1.png" {
			if string(data) != "new replaced image bytes" {
				t.Errorf("Esperado 'new replaced image bytes', obtido %q", string(data))
			}
		}
		if f.Name == "document.xml" {
			if string(data) != "xml content" {
				t.Errorf("Esperado 'xml content', obtido %q", string(data))
			}
		}
	}
}
