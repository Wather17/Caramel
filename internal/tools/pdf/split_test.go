package pdf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestSplitPDFCreatesOnePagePerFileByDefault(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.pdf")
	writePDFWithPageSizes(t, input, [][2]float64{{200, 300}, {400, 300}, {612, 792}})
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("não foi possível ler PDF de entrada: %v", err)
	}

	outputs, err := SplitPDF(input, "", nil)
	if err != nil {
		t.Fatalf("SplitPDF() falhou: %v", err)
	}
	if len(outputs) != 3 {
		t.Fatalf("SplitPDF() gerou %d saídas; esperava 3", len(outputs))
	}
	wantNames := []string{"lesson_page_001.pdf", "lesson_page_002.pdf", "lesson_page_003.pdf"}
	wantDims := [][2]float64{{200, 300}, {400, 300}, {612, 792}}
	for i, output := range outputs {
		if filepath.Base(output) != wantNames[i] {
			t.Errorf("saída %d chama-se %q; esperava %q", i+1, filepath.Base(output), wantNames[i])
		}
		dims, err := api.PageDimsFile(output)
		if err != nil {
			t.Fatalf("não foi possível ler saída %q: %v", output, err)
		}
		if len(dims) != 1 {
			t.Errorf("saída %q contém %d páginas; esperava 1", output, len(dims))
			continue
		}
		if !closeEnough(dims[0].Width, wantDims[i][0]) || !closeEnough(dims[0].Height, wantDims[i][1]) {
			t.Errorf("saída %q mede %.2fx%.2f; esperava %.2fx%.2f", output, dims[0].Width, dims[0].Height, wantDims[i][0], wantDims[i][1])
		}
	}
	after, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("não foi possível reler PDF de entrada: %v", err)
	}
	if string(after) != string(original) {
		t.Fatal("SplitPDF() alterou o PDF de entrada")
	}
}

func TestSplitPDFWritesRangesInSpecifiedOrder(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "book.pdf")
	sizes := [][2]float64{{100, 200}, {110, 210}, {120, 220}, {130, 230}, {140, 240}, {150, 250}}
	writePDFWithPageSizes(t, input, sizes)
	outputDir := filepath.Join(dir, "selected")
	spec := "4-5,1,6-6"

	outputs, err := SplitPDF(input, outputDir, &spec)
	if err != nil {
		t.Fatalf("SplitPDF() com intervalos falhou: %v", err)
	}
	wantNames := []string{"book_range_001_pages_004-005.pdf", "book_range_002_page_001.pdf", "book_range_003_page_006.pdf"}
	wantPageIndexes := [][]int{{3, 4}, {0}, {5}}
	if len(outputs) != len(wantNames) {
		t.Fatalf("SplitPDF() gerou %d saídas; esperava %d", len(outputs), len(wantNames))
	}
	for i, output := range outputs {
		if filepath.Base(output) != wantNames[i] {
			t.Errorf("saída %d chama-se %q; esperava %q", i+1, filepath.Base(output), wantNames[i])
		}
		dims, err := api.PageDimsFile(output)
		if err != nil {
			t.Fatalf("não foi possível ler saída %q: %v", output, err)
		}
		if len(dims) != len(wantPageIndexes[i]) {
			t.Fatalf("saída %q contém %d páginas; esperava %d", output, len(dims), len(wantPageIndexes[i]))
		}
		for pageIndex, sourceIndex := range wantPageIndexes[i] {
			if !closeEnough(dims[pageIndex].Width, sizes[sourceIndex][0]) || !closeEnough(dims[pageIndex].Height, sizes[sourceIndex][1]) {
				t.Errorf("saída %q página %d não corresponde à página de entrada %d", output, pageIndex+1, sourceIndex+1)
			}
		}
	}
}

func TestParsePageRangesAcceptsInclusiveRangesAndSinglePages(t *testing.T) {
	ranges, err := parsePageRanges(" 1-3, 5, 7-9 ", 9)
	if err != nil {
		t.Fatalf("parsePageRanges() falhou: %v", err)
	}
	want := []pageRange{{start: 1, end: 3}, {start: 5, end: 5}, {start: 7, end: 9}}
	if len(ranges) != len(want) {
		t.Fatalf("parser retornou %d intervalos; esperava %d", len(ranges), len(want))
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Errorf("intervalo %d = %+v; esperava %+v", i+1, ranges[i], want[i])
		}
	}
}

func TestSplitPDFRejectsInvalidRangesBeforeCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "six-pages.pdf")
	writePDFWithPageSizes(t, input, [][2]float64{{100, 200}, {110, 210}, {120, 220}, {130, 230}, {140, 240}, {150, 250}})
	invalidSpecs := []string{"", "0", "7", "3-2", "1-3,3-4", "1,,2", "x", "-1", "1-2-3"}

	for i, spec := range invalidSpecs {
		t.Run(spec, func(t *testing.T) {
			outputDir := filepath.Join(dir, "invalid", string(rune('a'+i)))
			if _, err := SplitPDF(input, outputDir, &spec); err == nil {
				t.Fatalf("SplitPDF() deveria rejeitar --ranges %q", spec)
			}
			if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
				t.Fatalf("pasta de saída deveria não existir para intervalo inválido; stat err=%v", err)
			}
		})
	}
}

func TestSplitPDFRejectsInvalidInputBeforeCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "invalid.pdf")
	if err := os.WriteFile(input, []byte("not a PDF"), 0o600); err != nil {
		t.Fatalf("não foi possível criar PDF inválido: %v", err)
	}
	outputDir := filepath.Join(dir, "invalid_split")

	if _, err := SplitPDF(input, outputDir, nil); err == nil {
		t.Fatal("SplitPDF() deveria rejeitar PDF inválido")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria existir para PDF inválido; stat err=%v", err)
	}
}
