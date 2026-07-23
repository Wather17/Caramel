package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ExtractText reads a .docx file and extracts all plain text, preserving paragraphs and basic layout.
func ExtractText(docxPath string) (string, error) {
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		return "", fmt.Errorf("failed to open docx file: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read word/document.xml: %w", err)
			}
			defer rc.Close()

			return parseDocumentXML(rc)
		}
	}

	return "", fmt.Errorf("word/document.xml not found in the docx archive")
}

// parseDocumentXML parses the document.xml file and extracts plain text, adding appropriate spacing.
func parseDocumentXML(reader io.Reader) (string, error) {
	decoder := xml.NewDecoder(reader)
	var buf bytes.Buffer
	var lastTag string

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error decoding XML: %w", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			name := se.Name.Local
			lastTag = name

			// Add a newline when entering a new paragraph (<w:p>) or table row (<w:tr>)
			if name == "p" || name == "tr" {
				// Don't prefix with double newlines unless needed
				if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") {
					buf.WriteByte('\n')
				}
			}
			// Add spaces between table cells (<w:tc>)
			if name == "tc" {
				if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") && !strings.HasSuffix(buf.String(), " ") {
					buf.WriteString(" | ")
				}
			}
			if name == "br" || name == "cr" {
				buf.WriteByte('\n')
			}
			if name == "tab" {
				buf.WriteByte('\t')
			}

		case xml.EndElement:
			name := se.Name.Local
			lastTag = "" // Reset lastTag to avoid capturing whitespace between tags
			// Add a newline when exiting a paragraph to ensure separation
			if name == "p" || name == "tr" {
				if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") {
					buf.WriteByte('\n')
				}
			}

		case xml.CharData:
			if lastTag == "t" {
				buf.Write(se)
			}
		}
	}

	// Clean up excess spacing or empty lines
	lines := strings.Split(buf.String(), "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	return strings.Join(cleanedLines, "\n"), nil
}
