package cli

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDOCXSplitCommandUsesDefaultOutputFolder(t *testing.T) {
	configureDOCXSplitTest(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.docx")
	writeDOCXSplitCommandFixture(t, input)

	oldOut := docxSplitCmd.OutOrStdout()
	var stdout bytes.Buffer
	docxSplitCmd.SetOut(&stdout)
	defer docxSplitCmd.SetOut(oldOut)
	if err := docxSplitCmd.RunE(docxSplitCmd, []string{input}); err != nil {
		t.Fatalf("docx split falhou: %v", err)
	}

	for _, name := range []string{"lesson_part_001.docx", "lesson_part_002.docx"} {
		if _, err := os.Stat(filepath.Join(dir, "lesson_split", name)); err != nil {
			t.Errorf("saída %q não foi criada: %v", name, err)
		}
	}
	if !strings.Contains(stdout.String(), "2 arquivo(s) gerado(s)") {
		t.Errorf("resumo da CLI não informa a quantidade de saídas: %q", stdout.String())
	}
}

func TestDOCXSplitCommandAcceptsOutputDirFlag(t *testing.T) {
	configureDOCXSplitTest(t)
	defer configureDOCXSplitTest(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "lesson.docx")
	writeDOCXSplitCommandFixture(t, input)
	outputDir := filepath.Join(dir, "chapters")
	if err := docxSplitCmd.Flags().Set("output-dir", outputDir); err != nil {
		t.Fatalf("não foi possível configurar --output-dir: %v", err)
	}

	if err := docxSplitCmd.RunE(docxSplitCmd, []string{input}); err != nil {
		t.Fatalf("docx split --output-dir falhou: %v", err)
	}
	for _, name := range []string{"lesson_part_001.docx", "lesson_part_002.docx"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Errorf("saída %q não foi criada: %v", name, err)
		}
	}
}

func configureDOCXSplitTest(t *testing.T) {
	t.Helper()
	docxSplitOutputDir = ""
	docxSplitCmd.Flags().Lookup("output-dir").Changed = false
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}

func writeDOCXSplitCommandFixture(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entries := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>primeira</w:t></w:r></w:p><w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>segunda</w:t></w:r></w:p><w:sectPr/></w:body></w:document>`,
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
