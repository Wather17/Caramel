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
