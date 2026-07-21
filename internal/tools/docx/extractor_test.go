package docx_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"caramel/internal/tools/docx"
)

// createMockDocx cria um arquivo .docx falso em memória com imagens simuladas
func createMockDocx(t *testing.T, filePath string, imageNames []string) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Adiciona [Content_Types].xml para simular estrutura docx
	w, err := zipWriter.Create("[Content_Types].xml")
	if err != nil {
		t.Fatalf("Erro ao criar [Content_Types].xml: %v", err)
	}
	w.Write([]byte("<Types></Types>"))

	// Adiciona imagens dentro de word/media/
	for _, imgName := range imageNames {
		w, err := zipWriter.Create("word/media/" + imgName)
		if err != nil {
			t.Fatalf("Erro ao criar imagem simulada %s: %v", imgName, err)
		}
		w.Write([]byte("fake image content for " + imgName))
	}

	zipWriter.Close()

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Erro ao escrever mock docx: %v", err)
	}
}

func TestListImages(t *testing.T) {
	tempDir := t.TempDir()
	mockDocxPath := filepath.Join(tempDir, "teste_atividade.docx")

	mockImages := []string{"image1.png", "figura2.jpg", "diagrama3.svg"}
	createMockDocx(t, mockDocxPath, mockImages)

	images, err := docx.ListImages(mockDocxPath)
	if err != nil {
		t.Fatalf("ListImages falhou: %v", err)
	}

	if len(images) != len(mockImages) {
		t.Errorf("Esperado %d imagens, obtido %d", len(mockImages), len(images))
	}

	for i, img := range images {
		if img.OriginalName != mockImages[i] {
			t.Errorf("Esperado nome %s, obtido %s", mockImages[i], img.OriginalName)
		}
	}
}

func TestExtractImages(t *testing.T) {
	tempDir := t.TempDir()
	mockDocxPath := filepath.Join(tempDir, "prova.docx")
	outDir := filepath.Join(tempDir, "extraidas")

	mockImages := []string{"grafico1.png", "mapa2.png"}
	createMockDocx(t, mockDocxPath, mockImages)

	result, err := docx.ExtractImages(mockDocxPath, outDir)
	if err != nil {
		t.Fatalf("ExtractImages falhou: %v", err)
	}

	if result.TotalExtracted != 2 {
		t.Errorf("Esperado 2 imagens extraídas, obtido %d", result.TotalExtracted)
	}

	for _, imgName := range mockImages {
		extractedPath := filepath.Join(outDir, imgName)
		if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
			t.Errorf("Arquivo extraído não encontrado no disco: %s", extractedPath)
		}
	}
}
