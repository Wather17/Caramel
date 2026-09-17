package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"
	"github.com/spf13/cobra"
)

func TestPDFMergeCommand(t *testing.T) {
	merge, _, err := RootCmd.Find([]string{"pdf", "merge"})
	if err != nil || merge == nil {
		t.Fatalf("comando pdf merge não foi encontrado: %v", err)
	}
	if merge.Flags().Lookup("output") == nil {
		t.Fatal("pdf merge deveria expor --output")
	}
	if _, required := merge.Flags().Lookup("output").Annotations[cobra.BashCompOneRequiredFlag]; !required {
		t.Fatal("pdf merge deveria exigir --output")
	}

	dir := t.TempDir()
	first := filepath.Join(dir, "first.pdf")
	second := filepath.Join(dir, "second.pdf")
	output := filepath.Join(dir, "merged.pdf")
	writeMergeFixturePDF(t, first)
	writeMergeFixturePDF(t, second)
	if err := merge.Flags().Set("output", output); err != nil {
		t.Fatalf("não foi possível configurar --output: %v", err)
	}
	defer func() { _ = merge.Flags().Set("output", "") }()

	var stdout bytes.Buffer
	merge.SetOut(&stdout)
	if err := merge.RunE(merge, []string{first, second}); err != nil {
		t.Fatalf("execução de pdf merge falhou: %v", err)
	}
	if !strings.Contains(stdout.String(), "2 arquivo(s), 2 página(s)") {
		t.Errorf("resumo não informa a quantidade de arquivos e páginas: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), output) {
		t.Errorf("resumo não informa o caminho de saída: %q", stdout.String())
	}
}

func writeMergeFixturePDF(t *testing.T, path string) {
	t.Helper()
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Text(10, 10, "fixture")
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatalf("não foi possível criar fixture PDF %q: %v", path, err)
	}
}
