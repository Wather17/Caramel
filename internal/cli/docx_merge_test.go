package cli

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDOCXMergeCommandUsesOutputFlagAndReportsResult(t *testing.T) {
	configureDOCXMergeTest(t)
	defer configureDOCXMergeTest(t)
	dir := t.TempDir()
	first := filepath.Join(dir, "first.docx")
	second := filepath.Join(dir, "second.docx")
	output := filepath.Join(dir, "merged.docx")
	writeCLIForMergeDOCX(t, first, "um")
	writeCLIForMergeDOCX(t, second, "dois")
	if err := docxMergeCmd.Flags().Set("output", output); err != nil {
		t.Fatalf("não foi possível configurar --output: %v", err)
	}

	oldOut := docxMergeCmd.OutOrStdout()
	var stdout bytes.Buffer
	docxMergeCmd.SetOut(&stdout)
	defer docxMergeCmd.SetOut(oldOut)
	if err := docxMergeCmd.RunE(docxMergeCmd, []string{first, second}); err != nil {
		t.Fatalf("docx merge falhou: %v", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("o DOCX final não foi criado: %v", err)
	}
	if !strings.Contains(stdout.String(), "DOCX juntados: 2 arquivo(s)") || !strings.Contains(stdout.String(), output) {
		t.Errorf("a saída da CLI não informa resumo e destino: %q", stdout.String())
	}
}

func TestDOCXMergeCommandRequiresTwoInputsAndOutput(t *testing.T) {
	configureDOCXMergeTest(t)
	if err := docxMergeCmd.Args(docxMergeCmd, []string{"one.docx"}); err == nil {
		t.Fatal("docx merge deveria exigir dois argumentos")
	}
	if err := docxMergeCmd.ValidateRequiredFlags(); err == nil {
		t.Fatal("docx merge deveria exigir --output/-o")
	}
}

func configureDOCXMergeTest(t *testing.T) {
	t.Helper()
	docxMergeOutput = ""
	docxMergeCmd.Flags().Lookup("output").Changed = false
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}

func writeCLIForMergeDOCX(t *testing.T, destination, text string) {
	t.Helper()
	file, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entries := map[string]string{
		"[Content_Types].xml": fmt.Sprintf(`<Types xmlns="%s"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`, "http://schemas.openxmlformats.org/package/2006/content-types"),
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   fmt.Sprintf(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>%s</w:t></w:r></w:p><w:sectPr/></w:body></w:document>`, text),
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
