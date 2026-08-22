package cards_test

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/tools/cards"
)

// writeRealPNG cria um PNG válido em disco (necessário para o gofpdf decodificar)
func writeRealPNG(t *testing.T, path string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 230, G: 60, B: 60, A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("falha ao criar PNG: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("falha ao codificar PNG: %v", err)
	}
}

func TestGenerateCardsHTML(t *testing.T) {
	tmpDir := t.TempDir()

	// Cria imagem fake para teste
	dummyImg := filepath.Join(tmpDir, "01_maca.png")
	if err := os.WriteFile(dummyImg, []byte("fake-png-bytes"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo de teste: %v", err)
	}

	items := []cards.CardItem{
		{Name: "Maçã", ImagePath: dummyImg},
		{Name: "Banana", ImagePath: dummyImg},
		{Name: "Morango", ImagePath: dummyImg},
		{Name: "Abacaxi", ImagePath: dummyImg},
		{Name: "Melancia", ImagePath: dummyImg},
		{Name: "Uva", ImagePath: dummyImg},
		{Name: "Laranja", ImagePath: dummyImg}, // 7º item -> vai para página 2
	}

	opts := cards.DefaultOptions()
	opts.Title = "Coleção de Frutas"
	opts.Columns = 2
	opts.Rows = 3

	outPath := filepath.Join(tmpDir, "fichas.html")
	err := cards.GenerateCardsHTML(items, outPath, opts)
	if err != nil {
		t.Fatalf("GenerateCardsHTML falhou: %v", err)
	}

	contentBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler HTML gerado: %v", err)
	}
	content := string(contentBytes)

	// Verificações
	if !strings.Contains(content, "Coleção de Frutas") {
		t.Errorf("HTML não contém o título esperado")
	}
	if !strings.Contains(content, "MAÇÃ") {
		t.Errorf("HTML não contém nome em maiúsculas")
	}
	if !strings.Contains(content, "Pág. 1 / 2") || !strings.Contains(content, "Pág. 2 / 2") {
		t.Errorf("HTML não contém a paginação esperada de 2 páginas")
	}
	if !strings.Contains(content, "data:image/png;base64,") {
		t.Errorf("HTML não contém imagem embutida em Base64")
	}
}

func TestGenerateCardsHTML_EmptyItems(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "empty.html")
	err := cards.GenerateCardsHTML([]cards.CardItem{}, outPath, cards.DefaultOptions())
	if err == nil {
		t.Errorf("esperava erro ao passar lista vazia de itens")
	}
}

func TestGenerateCardsPDF(t *testing.T) {
	tmpDir := t.TempDir()

	// Cria imagens reais para o gofpdf decodificar
	var items []cards.CardItem
	for i := 1; i <= 7; i++ { // 7 itens -> 2 páginas (grade 2x3)
		imgPath := filepath.Join(tmpDir, fmt.Sprintf("%02d_item.png", i))
		writeRealPNG(t, imgPath)
		items = append(items, cards.CardItem{Name: fmt.Sprintf("Item %d", i), ImagePath: imgPath})
	}

	opts := cards.DefaultOptions()
	opts.Title = "Coleção de Teste"
	opts.Columns = 2
	opts.Rows = 3

	outPath := filepath.Join(tmpDir, "fichas.pdf")
	err := cards.GenerateCardsPDF(items, outPath, opts)
	if err != nil {
		t.Fatalf("GenerateCardsPDF falhou: %v", err)
	}

	// Valida que é um PDF válido e não vazio
	stat, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("PDF não encontrado: %v", err)
	}
	if stat.Size() == 0 {
		t.Error("PDF gerado está com 0 bytes")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler PDF: %v", err)
	}
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Error("arquivo gerado não é um PDF válido (não começa com %PDF)")
	}
}

func TestGenerateCardsPDF_ImagemInexistente(t *testing.T) {
	tmpDir := t.TempDir()

	items := []cards.CardItem{
		{Name: "Maçã", ImagePath: filepath.Join(tmpDir, "nao_existe.png")},
	}

	outPath := filepath.Join(tmpDir, "fichas.pdf")
	err := cards.GenerateCardsPDF(items, outPath, cards.DefaultOptions())
	if err != nil {
		t.Fatalf("GenerateCardsPDF deveria ignorar imagem inexistente e gerar o PDF: %v", err)
	}

	stat, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("PDF não encontrado: %v", err)
	}
	if stat.Size() == 0 {
		t.Error("PDF gerado está com 0 bytes")
	}
}

func TestGenerateCardsPDF_EmptyItems(t *testing.T) {
	tmpDir := t.TempDir()
	err := cards.GenerateCardsPDF([]cards.CardItem{}, filepath.Join(tmpDir, "vazio.pdf"), cards.DefaultOptions())
	if err == nil {
		t.Errorf("esperava erro ao passar lista vazia de itens")
	}
}
