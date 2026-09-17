package docx_test

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/tools/docx"
)

const splitTestDocument = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" mc:Ignorable="w14">
  <w:body>
    <w:p w14:paraId="00000001"><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:t>antes</w:t></w:r><w:r><w:br w:type="page"/></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>depois</w:t></w:r></w:p>
    <w:p><w:pPr><w:pageBreakBefore/><w:sectPr><w:pgSz w:w="12240" w:h="15840"/></w:sectPr></w:pPr><w:r><w:t>terceiro</w:t></w:r></w:p>
    <w:p><w:r><w:t>quarto</w:t></w:r></w:p>
    <w:sectPr><w:pgSz w:w="12240" w:h="15840"/></w:sectPr>
  </w:body>
</w:document>`

func TestSplitDOCXUsesExplicitPageAndSectionBreaks(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.docx")
	writeSplitDOCXFixture(t, input, splitTestDocument)

	paths, err := docx.SplitDOCX(input, "")
	if err != nil {
		t.Fatalf("SplitDOCX falhou: %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("esperava 4 partes, recebeu %d: %v", len(paths), paths)
	}
	wantText := []string{"antes", "depois", "terceiro", "quarto"}
	for i, path := range paths {
		if filepath.Dir(path) != filepath.Join(dir, "lesson_split") {
			t.Errorf("parte %d foi gravada em pasta inesperada: %s", i+1, path)
		}
		if filepath.Base(path) != fmt.Sprintf("lesson_part_%03d.docx", i+1) {
			t.Errorf("nome inesperado para a parte %d: %s", i+1, path)
		}
		text, err := docx.ExtractText(path)
		if err != nil {
			t.Fatalf("não foi possível ler texto da parte %d: %v", i+1, err)
		}
		if strings.TrimSpace(text) != wantText[i] {
			t.Errorf("texto da parte %d: esperado %q, recebido %q", i+1, wantText[i], text)
		}
		assertSplitPackagePreservesParts(t, path)
	}
}

func TestSplitDOCXIgnoresRenderedPageBreakAndCreatesNoOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "rendered.docx")
	document := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>um</w:t><w:lastRenderedPageBreak/><w:t>dois</w:t></w:r></w:p></w:body></w:document>`
	writeSplitDOCXFixture(t, input, document)
	outputDir := filepath.Join(dir, "rendered_split")

	if _, err := docx.SplitDOCX(input, ""); err == nil {
		t.Fatal("SplitDOCX deveria rejeitar documento sem quebra explícita")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria ser criada: stat err=%v", err)
	}
}

func TestSplitDOCXRejectsBreaksInsideNestedBlocks(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "table.docx")
	document := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>antes</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>célula</w:t><w:br w:type="page"/><w:t>continuação</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p><w:r><w:t>depois</w:t></w:r></w:p></w:body></w:document>`
	writeSplitDOCXFixture(t, input, document)
	outputDir := filepath.Join(dir, "table_split")

	if _, err := docx.SplitDOCX(input, ""); err == nil || !strings.Contains(err.Error(), "blocos OOXML aninhados") {
		t.Fatalf("SplitDOCX deveria explicar que a quebra em tabela não é suportada; err=%v", err)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria ser criada: stat err=%v", err)
	}
}

func TestSplitDOCXIgnoresDisabledOrForeignPageBreakMarkers(t *testing.T) {
	tests := map[string]string{
		"disabled pageBreakBefore": `<w:p><w:pPr><w:pageBreakBefore w:val="false"/></w:pPr><w:r><w:t>um</w:t></w:r></w:p><w:p><w:r><w:t>dois</w:t></w:r></w:p>`,
		"foreign type attribute":   `<w:p><w:r><w:t>um</w:t><w:br xmlns:x="urn:example" x:type="page"/></w:r></w:p><w:p><w:r><w:t>dois</w:t></w:r></w:p>`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "markers.docx")
			document := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body + `</w:body></w:document>`
			writeSplitDOCXFixture(t, input, document)
			outputDir := filepath.Join(dir, "markers_split")

			if _, err := docx.SplitDOCX(input, ""); err == nil {
				t.Fatal("marcador desativado ou de outro namespace não deveria dividir o DOCX")
			}
			if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
				t.Fatalf("pasta de saída não deveria ser criada: stat err=%v", err)
			}
		})
	}
}

func TestSplitDOCXDoesNotTreatEscapedWhitespaceAsContent(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "whitespace.docx")
	document := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>&#x20;</w:t><w:br w:type="page"/></w:r></w:p><w:p><w:r><w:t>texto</w:t></w:r></w:p></w:body></w:document>`
	writeSplitDOCXFixture(t, input, document)
	outputDir := filepath.Join(dir, "whitespace_split")

	if _, err := docx.SplitDOCX(input, ""); err == nil {
		t.Fatal("espaço XML codificado não deveria gerar uma parte vazia")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria ser criada: stat err=%v", err)
	}
}

func TestSplitDOCXSkipsEmptySegmentsAndRejectsOutputCollisions(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "empty.docx")
	document := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:br w:type="page"/></w:r></w:p><w:p><w:r><w:t>um</w:t></w:r></w:p><w:p><w:r><w:br w:type="page"/></w:r></w:p><w:p><w:r><w:t>dois</w:t></w:r></w:p><w:sectPr/></w:body></w:document>`
	writeSplitDOCXFixture(t, input, document)
	outputDir := filepath.Join(dir, "parts")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(outputDir, "empty_part_002.docx")
	if err := os.WriteFile(collision, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := docx.SplitDOCX(input, outputDir); err == nil {
		t.Fatal("SplitDOCX deveria rejeitar colisão de saída")
	}
	if data, err := os.ReadFile(collision); err != nil || string(data) != "keep" {
		t.Fatalf("arquivo de destino existente foi alterado: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "empty_part_001.docx")); !os.IsNotExist(err) {
		t.Fatalf("não deveria haver saída parcial; stat err=%v", err)
	}

	if err := os.Remove(collision); err != nil {
		t.Fatal(err)
	}
	paths, err := docx.SplitDOCX(input, outputDir)
	if err != nil {
		t.Fatalf("SplitDOCX com segmentos vazios falhou: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("segmentos vazios não deveriam gerar arquivos: %v", paths)
	}
}

func TestSplitDOCXRejectsInvalidPackageWithoutCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "broken.docx")
	if err := os.WriteFile(input, []byte("não é um ZIP"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(dir, "broken_split")

	if _, err := docx.SplitDOCX(input, ""); err == nil {
		t.Fatal("SplitDOCX deveria rejeitar um arquivo inválido")
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("pasta de saída não deveria ser criada: stat err=%v", err)
	}
}

func writeSplitDOCXFixture(t *testing.T, path, document string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entries := map[string]string{
		"[Content_Types].xml":          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":            document,
		"word/styles.xml":              `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:styleId="Normal"><w:name w:val="Normal"/></w:style></w:styles>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/><Relationship Id="rIdImage" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`,
		"word/media/image1.png":        "image bytes",
	}
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertSplitPackagePreservesParts(t *testing.T, path string) {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("saída não é um ZIP válido: %v", err)
	}
	defer archive.Close()
	seen := map[string]bool{}
	for _, file := range archive.File {
		seen[file.Name] = true
		if file.Name != "word/document.xml" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, reader); err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"[Content_Types].xml", "word/_rels/document.xml.rels", "word/styles.xml", "word/media/image1.png"} {
		if !seen[name] {
			t.Errorf("pacote de saída perdeu a parte %q", name)
		}
	}
}
