package ui_test

import (
	"testing"

	_ "caramel/internal/cli" // registra a árvore de comandos real no guia
	"caramel/internal/ui"
)

func TestGetAllCommandDocs(t *testing.T) {
	docs := ui.GetAllCommandDocs()

	if len(docs) == 0 {
		t.Fatal("Esperado encontrar ao menos 1 documento de ajuda, obtido 0")
	}

	names := map[string]bool{}
	for _, doc := range docs {
		if doc.Name == "" {
			t.Error("Documento de comando encontrado com Nome vazio")
		}
		if names[doc.Name] {
			t.Errorf("Documento duplicado encontrado: %s", doc.Name)
		}
		names[doc.Name] = true

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

func TestGetAllCommandDocs_ComandosEsperados(t *testing.T) {
	docs := ui.GetAllCommandDocs()

	expected := map[string]bool{
		"caramel cards":          false,
		"caramel colorize":       false,
		"caramel process":        false,
		"caramel generate":       false,
		"caramel 2up":            false,
		"caramel docx extract":   false,
		"caramel routine process": false,
		"caramel install":        false,
		"caramel guide":          false,
		"caramel version":        false,
		"caramel config setup":   false,
		"caramel config set":     false,
		"caramel config show":    false,
	}

	for _, doc := range docs {
		if _, ok := expected[doc.Name]; ok {
			expected[doc.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("Comando '%s' não foi documentado no guia (pode não estar registrado no Cobra ou não ser runnable)", name)
		}
	}
}