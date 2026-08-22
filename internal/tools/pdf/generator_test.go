package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
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

func TestSortNatural(t *testing.T) {
	paths := []string{"img10.png", "img1.png", "img2.png", "img1_2.png", "img1_1.png"}
	SortNatural(paths)

	expected := []string{"img1.png", "img1_1.png", "img1_2.png", "img2.png", "img10.png"}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("Na posição %d esperava '%s', obteve '%s'", i, expected[i], p)
		}
	}
}

func TestCalculateFitDimensions(t *testing.T) {
	maxW := 138.5
	maxH := 200.0

	t.Run("Contain Landscape", func(t *testing.T) {
		rw, rh := calculateFitDimensions(800, 450, maxW, maxH, "contain")
		if rw > maxW || rh > maxH {
			t.Errorf("Contain excedeu o slot: %fx%f (max: %fx%f)", rw, rh, maxW, maxH)
		}
	})

	t.Run("Cover Landscape", func(t *testing.T) {
		rw, rh := calculateFitDimensions(800, 450, maxW, maxH, "cover")
		if rw < maxW && rh < maxH {
			t.Errorf("Cover não preencheu o slot: %fx%f (max: %fx%f)", rw, rh, maxW, maxH)
		}
	})
}

func TestGenerate2UpPDF_SmartLayout(t *testing.T) {
	tempDir := t.TempDir()
	landscapeImgPath := filepath.Join(tempDir, "landscape_activity.png")
	outPdfPath := filepath.Join(tempDir, "output_smart_2up.pdf")

	// Cria uma imagem PNG horizontal (landscape 16:9)
	img := image.NewRGBA(image.Rect(0, 0, 800, 450))
	for x := 0; x < 800; x++ {
		for y := 0; y < 450; y++ {
			img.Set(x, y, color.Black)
		}
	}

	f, err := os.Create(landscapeImgPath)
	if err != nil {
		t.Fatalf("Falha ao criar imagem landscape de teste: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Falha ao codificar PNG: %v", err)
	}
	f.Close()

	opts := DefaultOptions()
	opts.AutoRotate = true
	opts.RotateThreshold = 15.0
	opts.FitMode = "cover"

	err = Generate2UpPDF([]string{landscapeImgPath}, outPdfPath, opts)
	if err != nil {
		t.Fatalf("Generate2UpPDF com Smart Layout falhou: %v", err)
	}

	stat, err := os.Stat(outPdfPath)
	if err != nil {
		t.Fatalf("PDF gerado não encontrado: %v", err)
	}
	if stat.Size() == 0 {
		t.Errorf("O arquivo PDF com Smart Layout gerado está com 0 bytes")
	}
}

func TestOptimizeImageInMemory(t *testing.T) {
	tempDir := t.TempDir()
	largeImgPath := filepath.Join(tempDir, "large_figma_export.png")

	// Cria uma imagem PNG de altíssima resolução (ex: 5000x3000)
	img := image.NewRGBA(image.Rect(0, 0, 5000, 3000))
	for x := 0; x < 5000; x += 10 {
		for y := 0; y < 3000; y += 10 {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	f, err := os.Create(largeImgPath)
	if err != nil {
		t.Fatalf("Falha ao criar imagem grande de teste: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Falha ao codificar PNG grande: %v", err)
	}
	f.Close()

	opts := DefaultOptions()
	opts.Optimize = true
	opts.MaxDPI = 300
	opts.Quality = 85

	reader, format, err := optimizeImageInMemory(largeImgPath, 138.5, 200.0, opts)
	if err != nil {
		t.Fatalf("optimizeImageInMemory falhou: %v", err)
	}
	if format != "JPG" {
		t.Errorf("Esperava formato JPG, obteve %s", format)
	}
	if reader == nil {
		t.Errorf("Reader retornado é nil")
	}
}

// decodeJPEGReader decodifica o reader JPEG otimizado e retorna as dimensões reais
func decodeJPEGReader(t *testing.T, r io.Reader) (int, int) {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Falha ao ler JPEG otimizado: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Falha ao decodificar JPEG otimizado: %v", err)
	}
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

func TestOptimizeImageInMemory_SemRedimensionamentoDesnecessario(t *testing.T) {
	tempDir := t.TempDir()

	// Imagem retrato 1600x2200: cabe no slot 138.5x200mm a 300 DPI (limites 1635x2362).
	// Antes da correção, o swap de eixos fazia ela ser redimensionada ~32% sem necessidade.
	img := image.NewRGBA(image.Rect(0, 0, 1600, 2200))
	for y := 0; y < 2200; y += 10 {
		for x := 0; x < 1600; x += 10 {
			img.Set(x, y, color.RGBA{R: 30, G: 120, B: 200, A: 255})
		}
	}
	imgPath := filepath.Join(tempDir, "retrato_1600x2200.png")
	f, _ := os.Create(imgPath)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Falha ao codificar PNG: %v", err)
	}
	f.Close()

	opts := DefaultOptions()
	opts.Optimize = true

	// Dimensões visuais reais do encaixe (não o slot inteiro)
	reader, _, err := optimizeImageInMemory(imgPath, 138.5, 200.0, opts)
	if err != nil {
		t.Fatalf("optimizeImageInMemory falhou: %v", err)
	}

	w, h := decodeJPEGReader(t, reader)
	if w != 1600 || h != 2200 {
		t.Errorf("Esperado imagem NÃO redimensionada (1600x2200), obteve %dx%d", w, h)
	}
}

func TestOptimizeImageInMemory_LandscapeRedimensionaComAspecto(t *testing.T) {
	tempDir := t.TempDir()

	// Imagem landscape gigante: deve ser reduzida respeitando os limites de DPI e o aspecto
	img := image.NewRGBA(image.Rect(0, 0, 5000, 3000))
	for y := 0; y < 3000; y += 10 {
		for x := 0; x < 5000; x += 10 {
			img.Set(x, y, color.RGBA{R: 200, G: 120, B: 30, A: 255})
		}
	}
	imgPath := filepath.Join(tempDir, "landscape_5000x3000.png")
	f, _ := os.Create(imgPath)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Falha ao codificar PNG: %v", err)
	}
	f.Close()

	opts := DefaultOptions()
	opts.Optimize = true

	reader, _, err := optimizeImageInMemory(imgPath, 138.5, 200.0, opts)
	if err != nil {
		t.Fatalf("optimizeImageInMemory falhou: %v", err)
	}

	w, h := decodeJPEGReader(t, reader)

	// Limites a 300 DPI para 138.5x200mm: W<=1635 e H<=2362
	if w > 1635 || h > 2362 {
		t.Errorf("Imagem excedeu os limites de DPI: %dx%d (limites 1635x2362)", w, h)
	}

	// Aspecto original 5000/3000 = 1.6667 deve ser preservado (±0.01)
	aspect := float64(w) / float64(h)
	if aspect < 1.65 || aspect > 1.68 {
		t.Errorf("Aspecto distorcido: %d/%d = %.4f (esperado ~1.667)", w, h, aspect)
	}
}

func TestComputeRenderLayout(t *testing.T) {
	// Slot retrato (padrão do 2up A4 Paisagem): 138.5 x 200 mm
	const maxW = 138.5
	const maxH = 200.0

	t.Run("Landscape rotaciona e permanece dentro do slot", func(t *testing.T) {
		opts := DefaultOptions()
		layout := computeRenderLayout(2000, 1500, maxW, maxH, opts)

		if !layout.Rot {
			t.Fatal("Esperado rotação para imagem landscape 2000x1500 no slot retrato")
		}
		// Após a rotação, o encaixe visual deve caber no slot (sem estourar a folha)
		if layout.RW > maxW || layout.RH > maxH {
			t.Errorf("Encaixe visual estourou o slot: %fx%f (max: %fx%f)", layout.RW, layout.RH, maxW, maxH)
		}
		// O retângulo a desenhar no frame rotacionado deve ter as dimensões TROCADAS
		// (DrawW/DrawH invertidos em relação ao encaixe visual RW/RH)
		if layout.DrawW != layout.RH || layout.DrawH != layout.RW {
			t.Errorf("Retângulo rotacionado com dimensões erradas: Draw=%fx%f, Visual=%fx%f",
				layout.DrawW, layout.DrawH, layout.RW, layout.RH)
		}
	})

	t.Run("Retrato não rotaciona", func(t *testing.T) {
		opts := DefaultOptions()
		layout := computeRenderLayout(1000, 1500, maxW, maxH, opts)

		if layout.Rot {
			t.Error("Esperado SEM rotação para imagem retrato 1000x1500 no slot retrato")
		}
		if layout.DrawW > maxW || layout.DrawH > maxH {
			t.Errorf("Encaixe estourou o slot: %fx%f (max: %fx%f)", layout.DrawW, layout.DrawH, maxW, maxH)
		}
	})

	t.Run("Quadrado não rotaciona", func(t *testing.T) {
		opts := DefaultOptions()
		layout := computeRenderLayout(1000, 1000, maxW, maxH, opts)
		if layout.Rot {
			t.Error("Esperado SEM rotação para imagem quadrada")
		}
	})

	t.Run("Threshold alto impede rotação", func(t *testing.T) {
		opts := DefaultOptions()
		opts.RotateThreshold = 200.0
		layout := computeRenderLayout(2000, 1500, maxW, maxH, opts)
		if layout.Rot {
			t.Error("Esperado SEM rotação com threshold de ganho muito alto")
		}
	})

	t.Run("Cover mantém imagem dentro do slot", func(t *testing.T) {
		opts := DefaultOptions()
		opts.FitMode = "cover"
		layout := computeRenderLayout(800, 450, maxW, maxH, opts)
		if layout.RW < maxW || layout.RH < maxH {
			t.Errorf("Cover deveria preencher o slot: %fx%f (slot %fx%f)", layout.RW, layout.RH, maxW, maxH)
		}
	})
}
