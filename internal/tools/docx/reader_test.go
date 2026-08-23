package docx

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDocumentXML(t *testing.T) {
	xmlInput := `
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
    <w:body>
        <w:p>
            <w:r>
                <w:t>Hello</w:t>
            </w:r>
            <w:r>
                <w:t> World</w:t>
            </w:r>
        </w:p>
        <w:tbl>
            <w:tr>
                <w:tc>
                    <w:p>
                        <w:t>Cell 1</w:t>
                    </w:p>
                </w:tc>
                <w:tc>
                    <w:p>
                        <w:t>Cell 2</w:t>
                    </w:p>
                </w:tc>
            </w:tr>
        </w:tbl>
    </w:body>
</w:document>
`
	reader := strings.NewReader(xmlInput)
	result, err := parseDocumentXML(reader)
	if err != nil {
		t.Fatalf("parseDocumentXML failed: %v", err)
	}

	expectedParts := []string{"Hello World", "Cell 1", "Cell 2"}
	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("expected output to contain %q, got: %q", part, result)
		}
	}
}

func TestExtractText(t *testing.T) {
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "atividade.docx")

	var buf strings.Builder
	zw := zip.NewWriter(&stringWriter{&buf})
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("falha ao criar document.xml: %v", err)
	}
	w.Write([]byte(`<w:document><w:body><w:p><w:r><w:t>Primeiro parágrafo</w:t></w:r></w:p><w:p><w:r><w:t>Segundo</w:t></w:r></w:p></w:body></w:document>`))
	zw.Close()

	if err := os.WriteFile(docxPath, []byte(buf.String()), 0644); err != nil {
		t.Fatalf("falha ao escrever docx: %v", err)
	}

	text, err := ExtractText(docxPath)
	if err != nil {
		t.Fatalf("ExtractText falhou: %v", err)
	}
	if !strings.Contains(text, "Primeiro parágrafo") || !strings.Contains(text, "Segundo") {
		t.Errorf("texto extraído incompleto: %q", text)
	}

	// Zip sem word/document.xml deve retornar erro
	semDoc := filepath.Join(dir, "semdoc.docx")
	buf.Reset()
	zw2 := zip.NewWriter(&stringWriter{&buf})
	w2, _ := zw2.Create("[Content_Types].xml")
	w2.Write([]byte("<xml/>"))
	zw2.Close()
	os.WriteFile(semDoc, []byte(buf.String()), 0644)

	if _, err := ExtractText(semDoc); err == nil {
		t.Error("zip sem document.xml deveria retornar erro")
	}

	if _, err := ExtractText(filepath.Join(dir, "inexistente.docx")); err == nil {
		t.Error("arquivo inexistente deveria retornar erro")
	}
}

// stringWriter adapta strings.Builder para io.Writer usado pelo zip writer
type stringWriter struct {
	sb *strings.Builder
}

func (s *stringWriter) Write(p []byte) (int, error) { return s.sb.Write(p) }
