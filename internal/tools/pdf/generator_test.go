package pdf

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareImagePairs(t *testing.T) {
	t.Run("Single Image Duplication", func(t *testing.T) {
		images := []string{"img1.png"}
		pairs := prepareImagePairs(images, true)
		if len(pairs) != 1 {
			t.Fatalf("Esperava 1 par, obteve %d", len(pairs))
		}
		if pairs[0].Left != "img1.png" || pairs[0].Right != "img1.png" {
			t.Errorf("Esperava duplicar img1.png em ambos os slots, obteve Left=%s, Right=%s", pairs[0].Left, pairs[0].Right)
		}
	})

	t.Run("Even Images Count", func(t *testing.T) {
		images := []string{"img1.png", "img2.png", "img3.png", "img4.png"}
		pairs := prepareImagePairs(images, true)
		if len(pairs) != 2 {
			t.Fatalf("Esperava 2 pares, obteve %d", len(pairs))
		}
		if pairs[0].Left != "img1.png" || pairs[0].Right != "img2.png" {
			t.Errorf("Par 0 incorreto")
		}
		if pairs[1].Left != "img3.png" || pairs[1].Right != "img4.png" {
			t.Errorf("Par 1 incorreto")
		}
	})

	t.Run("Odd Images Count With Duplication", func(t *testing.T) {
		images := []string{"img1.png", "img2.png", "img3.png"}
		pairs := prepareImagePairs(images, true)
		if len(pairs) != 2 {
			t.Fatalf("Esperava 2 pares, obteve %d", len(pairs))
		}
		if pairs[1].Left != "img3.png" || pairs[1].Right != "img3.png" {
			t.Errorf("Esperava duplicar a última imagem ímpar no slot direito, obteve Left=%s, Right=%s", pairs[1].Left, pairs[1].Right)
		}
	})
}

func TestGenerate2UpPDF(t *testing.T) {
	tempDir := t.TempDir()
	testImgPath := filepath.Join(tempDir, "test_activity.png")
	outPdfPath := filepath.Join(tempDir, "output_2up.pdf")

	// Cria uma imagem PNG simples para o teste
	img := image.NewRGBA(image.Rect(0, 0, 400, 600))
	for x := 0; x < 400; x++ {
		for y := 0; y < 600; y++ {
			img.Set(x, y, color.White)
		}
	}

	f, err := os.Create(testImgPath)
	if err != nil {
		t.Fatalf("Falha ao criar imagem de teste: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Falha ao codificar PNG: %v", err)
	}
	f.Close()

	opts := DefaultOptions()
	opts.DrawCutLine = true

	err = Generate2UpPDF([]string{testImgPath}, outPdfPath, opts)
	if err != nil {
		t.Fatalf("Generate2UpPDF falhou: %v", err)
	}

	// Verifica se o PDF foi criado com sucesso e não está vazio
	stat, err := os.Stat(outPdfPath)
	if err != nil {
		t.Fatalf("PDF gerado não foi encontrado: %v", err)
	}
	if stat.Size() == 0 {
		t.Errorf("O arquivo PDF gerado está com 0 bytes")
	}
}
