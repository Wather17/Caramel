package ui_test

import (
	"strings"
	"testing"

	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

func TestRenderStyledHelp(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "caramel docx extract <arquivo.docx>",
		Short: "Extrai imagens do arquivo .docx",
		Long:  "Conjunto de utilitários para extrair e tratar figuras em documentos Word.",
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	output := ui.RenderStyledHelp(cmd)

	if !strings.Contains(output, "Caramel CLI") {
		t.Errorf("A saída formatada deveria conter o cabeçalho 'Caramel CLI'")
	}

	if !strings.Contains(output, "SINTAXE DE USO") {
		t.Errorf("A saída formatada deveria conter a seção 'SINTAXE DE USO'")
	}
}

func TestSearchCommandDocs(t *testing.T) {
	t.Run("Busca por Figma", func(t *testing.T) {
		results := ui.SearchCommandDocs("figma")
		if len(results) == 0 {
			t.Errorf("Esperava encontrar resultados para 'figma'")
		}
		found2Up := false
		for _, doc := range results {
			if doc.Name == "caramel 2up" {
				found2Up = true
				break
			}
		}
		if !found2Up {
			t.Errorf("Esperava encontrar 'caramel 2up' nos resultados de 'figma'")
		}
	})

	t.Run("Busca por Papel", func(t *testing.T) {
		results := ui.SearchCommandDocs("papel")
		if len(results) == 0 {
			t.Errorf("Esperava encontrar resultados para 'papel'")
		}
	})

	t.Run("Busca por Termo Inexistente", func(t *testing.T) {
		results := ui.SearchCommandDocs("termoinvalidoxyz123")
		if len(results) != 0 {
			t.Errorf("Esperava 0 resultados para termo inexistente, obteve %d", len(results))
		}
	})

	t.Run("Busca por Nome de Flag", func(t *testing.T) {
		results := ui.SearchCommandDocs("custom-style")
		if len(results) == 0 {
			t.Errorf("Esperava encontrar resultados ao buscar por nome de flag 'custom-style'")
		}
		foundGenerate := false
		for _, doc := range results {
			if doc.Name == "caramel generate" {
				foundGenerate = true
				break
			}
		}
		if !foundGenerate {
			t.Errorf("Esperava encontrar 'caramel generate' na busca por 'custom-style'")
		}
	})
}

func TestRenderSearchHelp(t *testing.T) {
	out := ui.RenderSearchHelp("2up")
	if !strings.Contains(out, "caramel 2up") {
		t.Errorf("RenderSearchHelp para '2up' deveria conter 'caramel 2up'")
	}

	outEmpty := ui.RenderSearchHelp("termoinvalidoxyz123")
	if !strings.Contains(outEmpty, "Nenhum comando ou caso de uso encontrado") {
		t.Errorf("RenderSearchHelp para termo inexistente deveria indicar 'Nenhum comando encontrado'")
	}
}
