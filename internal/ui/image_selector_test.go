package ui

import (
	"path/filepath"
	"testing"

	"caramel/internal/tools/docx"
)

func TestSelectImagesInteractive_Empty(t *testing.T) {
	result, err := SelectImagesInteractive(nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if result != nil {
		t.Errorf("esperava nil para lista vazia, obteve %v", result)
	}

	resultEmpty, err := SelectImagesInteractive([]docx.ExtractedImage{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if resultEmpty != nil {
		t.Errorf("esperava nil para slice vazio, obteve %v", resultEmpty)
	}
}

func TestSelectImageFilesWithPreviewInteractive_EdgeCases(t *testing.T) {
	// Caso lista vazia
	resEmpty, err := SelectImageFilesWithPreviewInteractive(nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if resEmpty != nil {
		t.Errorf("esperava nil para lista vazia, obteve %v", resEmpty)
	}

	// Caso com apenas 1 imagem (deve retornar imediatamente sem abrir formulário)
	singlePath := []string{filepath.Join("tmp", "imagem.png")}
	resSingle, err := SelectImageFilesWithPreviewInteractive(singlePath)
	if err != nil {
		t.Fatalf("erro inesperado em imagem única: %v", err)
	}
	if len(resSingle) != 1 || resSingle[0] != singlePath[0] {
		t.Errorf("esperava %v, obteve %v", singlePath, resSingle)
	}
}
