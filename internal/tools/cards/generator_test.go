package cards_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/tools/cards"
)

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
