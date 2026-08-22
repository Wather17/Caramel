package ai_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"caramel/internal/tools/ai"
)

// createTestPNG grava uma imagem PNG de teste no disco e retorna seu caminho
func createTestPNG(t *testing.T, img image.Image) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "teste.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("falha ao criar PNG de teste: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("falha ao codificar PNG de teste: %v", err)
	}
	return path
}

// solidImage gera uma imagem retangular preenchida com uma única cor
func solidImage(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestIsLikelyAlreadyColored_ImagemColorida(t *testing.T) {
	// Metade vermelho vivo, metade azul vivo: 100% dos pixels saturados
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if x < 50 {
				img.Set(x, y, color.RGBA{R: 220, G: 20, B: 20, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 20, G: 20, B: 220, A: 255})
			}
		}
	}
	path := createTestPNG(t, img)

	colored, ratio, err := ai.IsLikelyAlreadyColored(path)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !colored {
		t.Errorf("esperado alreadyColored=true para imagem colorida, ratio obtido: %.4f", ratio)
	}
	if ratio < 0.9 {
		t.Errorf("esperado ratio próximo de 1.0 para imagem totalmente colorida, obtido: %.4f", ratio)
	}
}

func TestIsLikelyAlreadyColored_GradienteCinza(t *testing.T) {
	// Gradiente do preto ao branco: nenhum pixel saturado
	img := image.NewRGBA(image.Rect(0, 0, 256, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 256; x++ {
			v := uint8(x)
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	path := createTestPNG(t, img)

	colored, _, err := ai.IsLikelyAlreadyColored(path)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if colored {
		t.Error("esperado alreadyColored=false para gradiente em tons de cinza")
	}
}

func TestIsLikelyAlreadyColored_TextoPretoEmBranco(t *testing.T) {
	// Folha branca com "linhas de texto" pretas: simula imagem só com texto
	img := solidImage(200, 200, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	for y := 20; y < 200; y += 20 {
		for x := 20; x < 180; x++ {
			for dy := 0; dy < 4; dy++ {
				img.Set(x, y+dy, color.RGBA{A: 255})
			}
		}
	}
	path := createTestPNG(t, img)

	colored, _, err := ai.IsLikelyAlreadyColored(path)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if colored {
		t.Error("esperado alreadyColored=false para imagem de texto preto sobre branco")
	}
}

func TestIsLikelyAlreadyColored_ImagemTodaPreta(t *testing.T) {
	path := createTestPNG(t, solidImage(50, 50, color.RGBA{A: 255}))

	colored, _, err := ai.IsLikelyAlreadyColored(path)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if colored {
		t.Error("esperado alreadyColored=false para imagem toda preta (saturação indefinida deve ser 0)")
	}
}

func TestIsLikelyAlreadyColored_ManchaColoridaPequena(t *testing.T) {
	// Fundo cinza com apenas 1% de pixels coloridos: abaixo do threshold de 5%
	img := solidImage(100, 100, color.RGBA{R: 128, G: 128, B: 128, A: 255})
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	path := createTestPNG(t, img)

	colored, ratio, err := ai.IsLikelyAlreadyColored(path)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if colored {
		t.Errorf("esperado alreadyColored=false para apenas 1%% de pixels coloridos, ratio: %.4f", ratio)
	}
}

func TestIsLikelyAlreadyColored_ArquivoInexistente(t *testing.T) {
	_, _, err := ai.IsLikelyAlreadyColored(filepath.Join(t.TempDir(), "nao_existe.png"))
	if err == nil {
		t.Error("esperado erro para arquivo inexistente")
	}
}

func TestIsLikelyAlreadyColored_ArquivoInvalido(t *testing.T) {
	// Um arquivo que não é imagem decodificável (ex: SVG/texto) deve retornar erro (fail-open no caller)
	path := filepath.Join(t.TempDir(), "falso.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo de teste: %v", err)
	}

	_, _, err := ai.IsLikelyAlreadyColored(path)
	if err == nil {
		t.Error("esperado erro ao decodificar arquivo que não é imagem raster")
	}
}
