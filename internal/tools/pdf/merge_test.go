package pdf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/phpdave11/gofpdf"
)

func TestMergePDFsPreservesOrderAndPageDimensions(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.pdf")
	second := filepath.Join(dir, "second.pdf")
	merged := filepath.Join(dir, "merged.pdf")

	writePDFWithPageSizes(t, first, [][2]float64{{200, 300}, {400, 300}})
	writePDFWithPageSizes(t, second, [][2]float64{{612, 792}})

	count, err := MergePDFs([]string{first, second}, merged)
	if err != nil {
		t.Fatalf("MergePDFs() falhou: %v", err)
	}
	if count != 3 {
		t.Fatalf("MergePDFs() retornou %d páginas; esperava 3", count)
	}

	dims, err := api.PageDimsFile(merged)
	if err != nil {
		t.Fatalf("não foi possível ler as dimensões do PDF combinado: %v", err)
	}
	want := [][2]float64{{200, 300}, {400, 300}, {612, 792}}
	if len(dims) != len(want) {
		t.Fatalf("PDF combinado tem %d páginas; esperava %d", len(dims), len(want))
	}
	for i, expected := range want {
		if !closeEnough(dims[i].Width, expected[0]) || !closeEnough(dims[i].Height, expected[1]) {
			t.Errorf("página %d tem dimensões %.2fx%.2f; esperava %.2fx%.2f", i+1, dims[i].Width, dims[i].Height, expected[0], expected[1])
		}
	}
}

func TestMergePDFsRejectsInvalidInput(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.pdf")
	if err := os.WriteFile(invalid, []byte("not a PDF"), 0o600); err != nil {
		t.Fatalf("não foi possível criar fixture inválido: %v", err)
	}
	valid := filepath.Join(dir, "valid.pdf")
	writePDFWithPageSizes(t, valid, [][2]float64{{612, 792}})
	output := filepath.Join(dir, "out.pdf")

	if _, err := MergePDFs([]string{invalid, valid}, output); err == nil {
		t.Fatal("MergePDFs() deveria rejeitar um PDF inválido")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("arquivo de saída não deveria existir após entrada inválida; stat err=%v", err)
	}
}

func TestMergePDFsRejectsMissingInput(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.pdf")
	writePDFWithPageSizes(t, valid, [][2]float64{{612, 792}})
	missing := filepath.Join(dir, "missing.pdf")

	if _, err := MergePDFs([]string{valid, missing}, filepath.Join(dir, "out.pdf")); err == nil {
		t.Fatal("MergePDFs() deveria rejeitar arquivo de entrada inexistente")
	}
}

func TestMergePDFsRejectsOutputCollision(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.pdf")
	second := filepath.Join(dir, "second.pdf")
	writePDFWithPageSizes(t, first, [][2]float64{{612, 792}})
	writePDFWithPageSizes(t, second, [][2]float64{{200, 300}})
	original, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("não foi possível ler fixture: %v", err)
	}

	if _, err := MergePDFs([]string{first, second}, first); err == nil {
		t.Fatal("MergePDFs() deveria rejeitar saída igual a uma entrada")
	}
	after, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("não foi possível reler fixture: %v", err)
	}
	if string(after) != string(original) {
		t.Fatal("MergePDFs() alterou o arquivo de entrada após colisão")
	}
}

func TestMergePDFsRequiresTwoInputsAndOutput(t *testing.T) {
	if _, err := MergePDFs([]string{"one.pdf"}, "out.pdf"); err == nil {
		t.Fatal("MergePDFs() deveria exigir pelo menos dois arquivos")
	}
	if _, err := MergePDFs([]string{"one.pdf", "two.pdf"}, " "); err == nil {
		t.Fatal("MergePDFs() deveria exigir o caminho de saída")
	}
}

func writePDFWithPageSizes(t *testing.T, path string, sizes [][2]float64) {
	t.Helper()
	pdf := gofpdf.New("P", "pt", "A4", "")
	for i, size := range sizes {
		pdf.AddPageFormat("P", gofpdf.SizeType{Wd: size[0], Ht: size[1]})
		pdf.SetFont("Helvetica", "", 12)
		pdf.Text(20, 30, string(rune('A'+i)))
	}
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatalf("não foi possível criar fixture PDF %q: %v", path, err)
	}
}

func closeEnough(actual, expected float64) bool {
	const epsilon = 0.1
	return actual >= expected-epsilon && actual <= expected+epsilon
}
