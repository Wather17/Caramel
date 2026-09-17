package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"
)

func TestPDFSplitCommandUsesDefaultOutputFolder(t *testing.T) {
	configurePDFSplitTestFlags(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.pdf")
	writePDFSplitFixture(t, input, 3)

	stdout, err := runPDFSplitForTest(t, input)
	if err != nil {
		t.Fatalf("pdf split falhou: %v", err)
	}
	outputDir := filepath.Join(dir, "lesson_split")
	for page := 1; page <= 3; page++ {
		path := filepath.Join(outputDir, fmt.Sprintf("lesson_page_%03d.pdf", page))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("saída da página %d não foi criada em %q: %v", page, path, err)
		}
	}
	if !strings.Contains(stdout, "3 arquivo(s) gerado(s)") {
		t.Errorf("resumo da CLI não informa a quantidade de saídas: %q", stdout)
	}
}

func TestPDFSplitCommandUsesRangesAndCustomOutputDirectory(t *testing.T) {
	configurePDFSplitTestFlags(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "book.pdf")
	writePDFSplitFixture(t, input, 6)
	outputDir := filepath.Join(dir, "chapters")
	if err := pdfSplitCmd.Flags().Set("ranges", "1-3,5,6"); err != nil {
		t.Fatalf("não foi possível configurar --ranges: %v", err)
	}
	if err := pdfSplitCmd.Flags().Set("output-dir", outputDir); err != nil {
		t.Fatalf("não foi possível configurar --output-dir: %v", err)
	}

	stdout, err := runPDFSplitForTest(t, input)
	if err != nil {
		t.Fatalf("pdf split com intervalos falhou: %v", err)
	}
	for _, name := range []string{"book_range_001_pages_001-003.pdf", "book_range_002_page_005.pdf", "book_range_003_page_006.pdf"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Errorf("saída %q não foi criada: %v", name, err)
		}
	}
	if !strings.Contains(stdout, "3 arquivo(s) gerado(s)") {
		t.Errorf("resumo da CLI não informa a quantidade de saídas: %q", stdout)
	}
}

func TestPDFSplitCommandRejectsExplicitEmptyRanges(t *testing.T) {
	configurePDFSplitTestFlags(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "book.pdf")
	writePDFSplitFixture(t, input, 2)
	outputDir := filepath.Join(dir, "empty_ranges")
	if err := pdfSplitCmd.Flags().Set("ranges", ""); err != nil {
		t.Fatalf("não foi possível configurar --ranges vazio: %v", err)
	}
	if err := pdfSplitCmd.Flags().Set("output-dir", outputDir); err != nil {
		t.Fatalf("não foi possível configurar --output-dir: %v", err)
	}

	if _, err := runPDFSplitForTest(t, input); err == nil {
		t.Fatal("pdf split deveria rejeitar --ranges vazio quando informado")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria ser criada para --ranges vazio; stat err=%v", err)
	}
}

func configurePDFSplitTestFlags(t *testing.T) {
	t.Helper()
	pdfSplitRanges = ""
	pdfSplitOutputDir = ""
	pdfSplitCmd.Flags().Lookup("ranges").Changed = false
	pdfSplitCmd.Flags().Lookup("output-dir").Changed = false
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}

func runPDFSplitForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	oldOut := pdfSplitCmd.OutOrStdout()
	var stdout bytes.Buffer
	pdfSplitCmd.SetOut(&stdout)
	defer pdfSplitCmd.SetOut(oldOut)
	err := pdfSplitCmd.RunE(pdfSplitCmd, args)
	return stdout.String(), err
}

func writePDFSplitFixture(t *testing.T, path string, pageCount int) {
	t.Helper()
	pdf := gofpdf.New("P", "pt", "A4", "")
	for page := 0; page < pageCount; page++ {
		pdf.AddPageFormat("P", gofpdf.SizeType{Wd: 200 + float64(page*10), Ht: 300 + float64(page*10)})
		pdf.SetFont("Helvetica", "", 12)
		pdf.Text(20, 30, "page")
	}
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatalf("não foi possível criar PDF fixture %q: %v", path, err)
	}
}
