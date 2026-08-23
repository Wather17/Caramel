package ui_test

import (
	"strings"
	"testing"

	_ "caramel/internal/cli" // registra a árvore de comandos real no guia
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
			if doc.Name == "caramel print 2up" {
				found2Up = true
				break
			}
		}
		if !found2Up {
			t.Errorf("Esperava encontrar 'caramel print 2up' nos resultados de 'figma'")
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
			if doc.Name == "caramel image generate" {
				foundGenerate = true
				break
			}
		}
		if !foundGenerate {
			t.Errorf("Esperava encontrar 'caramel image generate' na busca por 'custom-style'")
		}
	})

	t.Run("Busca por contexto pedagógico", func(t *testing.T) {
		results := ui.SearchCommandDocs("caça-palavras")
		if len(results) == 0 {
			t.Errorf("Esperava encontrar resultados para termo do contexto pedagógico 'caça-palavras'")
		}
	})
}

func TestRenderSearchHelp(t *testing.T) {
	out := ui.RenderSearchHelp("2up")
	if !strings.Contains(out, "caramel print 2up") {
		t.Errorf("RenderSearchHelp para '2up' deveria conter 'caramel print 2up'")
	}

	outEmpty := ui.RenderSearchHelp("termoinvalidoxyz123")
	if !strings.Contains(outEmpty, "Nenhum comando ou caso de uso encontrado") {
		t.Errorf("RenderSearchHelp para termo inexistente deveria indicar 'Nenhum comando encontrado'")
	}
}

func TestRenderGuideOverview(t *testing.T) {
	out := ui.RenderGuideOverview()

	if !strings.Contains(out, "Guia Didático") {
		t.Errorf("Overview deveria conter o cabeçalho 'Guia Didático'")
	}
	if !strings.Contains(out, "caramel print cards") {
		t.Errorf("Overview deveria listar 'caramel print cards'")
	}
	if !strings.Contains(out, "caramel image generate") {
		t.Errorf("Overview deveria listar 'caramel image generate'")
	}
	if !strings.Contains(out, "Configurações") {
		t.Errorf("Overview deveria conter a categoria de Configurações")
	}
}
func TestRenderStyledHelpGrupoComSubcomandos(t *testing.T) {
	sub := &cobra.Command{
		Use:   "extract <arquivo.docx>",
		Short: "Extrai imagens",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	group := &cobra.Command{
		Use:     "docx",
		Example: "caramel docx extract arquivo.docx",
	}
	group.AddCommand(sub)

	output := ui.RenderStyledHelp(group)

	if !strings.Contains(output, "SUBCOMANDOS DISPONÍVEIS") {
		t.Errorf("grupo deveria listar subcomandos, obtido: %s", output)
	}
	if !strings.Contains(output, "extract") {
		t.Errorf("subcomando 'extract' deveria aparecer na listagem, obtido: %s", output)
	}
	if !strings.Contains(output, "EXEMPLOS") || !strings.Contains(output, "arquivo.docx") {
		t.Errorf("exemplos do comando deveriam ser renderizados, obtido: %s", output)
	}
}

func TestRenderStyledHelpSemLongUsaShort(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Mostra a versão",
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	output := ui.RenderStyledHelp(cmd)
	if !strings.Contains(output, "Mostra a versão") {
		t.Errorf("sem Long, a Short deveria ser usada como descrição, obtido: %s", output)
	}
}
