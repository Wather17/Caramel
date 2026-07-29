package ui_test

import (
	"testing"

	"caramel/internal/ui"
)

func TestGetAllCommandDocs(t *testing.T) {
	docs := ui.GetAllCommandDocs()

	if len(docs) == 0 {
		t.Fatal("Esperado encontrar ao menos 1 documento de ajuda, obtido 0")
	}

	for _, doc := range docs {
		if doc.Name == "" {
			t.Error("Documento de comando encontrado com Nome vazio")
		}
		if doc.Short == "" {
			t.Errorf("Comando %s possui Short description vazia", doc.Name)
		}
		if doc.Category == "" {
			t.Errorf("Comando %s possui Categoria vazia", doc.Name)
		}
		if doc.Syntax == "" {
			t.Errorf("Comando %s possui Sintaxe vazia", doc.Name)
		}
		if len(doc.Examples) == 0 {
			t.Errorf("Comando %s não possui nenhum exemplo prático", doc.Name)
		}
	}
}
