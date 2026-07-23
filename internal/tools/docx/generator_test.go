package docx_test

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"caramel/internal/tools/docx"
)

func TestGeneratePedagogicalReport(t *testing.T) {
	rows := []docx.RoutineRow{
		{Data: "10/05/26", Campo: "O eu, o outro e o nós.", Experiencia: "Conversa em roda e brincadeiras coletivas."},
		{Data: "11/05/26", Campo: "Traços, sons, cores e formas.", Experiencia: "Pintura de guache livre nas mesas."},
	}

	docxBytes, err := docx.GeneratePedagogicalReport(rows)
	if err != nil {
		t.Fatalf("GeneratePedagogicalReport failed: %v", err)
	}

	if len(docxBytes) == 0 {
		t.Fatal("expected non-empty docx bytes")
	}

	// Verify the output is a valid ZIP and contains the correct data inside word/document.xml
	reader := bytes.NewReader(docxBytes)
	zipReader, err := zip.NewReader(reader, int64(len(docxBytes)))
	if err != nil {
		t.Fatalf("output is not a valid zip: %v", err)
	}

	var foundDocumentXML bool
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			foundDocumentXML = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open word/document.xml in generated docx: %v", err)
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("failed to read word/document.xml in generated docx: %v", err)
			}

			xmlStr := string(content)
			expectedStrings := []string{
				"10/05/26", "O eu, o outro e o nós.", "Conversa em roda",
				"11/05/26", "Traços, sons, cores e formas.", "Pintura de guache",
			}

			for _, s := range expectedStrings {
				if !strings.Contains(xmlStr, s) {
					t.Errorf("expected generated XML to contain %q", s)
				}
			}
		}
	}

	if !foundDocumentXML {
		t.Error("word/document.xml was not found in the generated docx archive")
	}
}
