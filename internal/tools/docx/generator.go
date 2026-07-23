package docx

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"io"
	"strings"
)

//go:embed templates/blank_landscape.docx
var blankLandscapeTemplate []byte

// RoutineRow represents a single row in the pedagogical routine report
type RoutineRow struct {
	Data        string `json:"data"`
	Campo       string `json:"campo"`
	Experiencia string `json:"experiencia"`
}

// GeneratePedagogicalReport creates a new landscape DOCX document filled with routine rows
func GeneratePedagogicalReport(rows []RoutineRow) ([]byte, error) {
	// 1. Open the embedded blank template zip
	templateReader := bytes.NewReader(blankLandscapeTemplate)
	zipReader, err := zip.NewReader(templateReader, int64(len(blankLandscapeTemplate)))
	if err != nil {
		return nil, fmt.Errorf("failed to open embedded docx template: %w", err)
	}

	// 2. Create a buffer to write the new docx zip
	var outputBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&outputBuffer)

	// 3. Process files in the template
	for _, f := range zipReader.File {
		header := &zip.FileHeader{
			Name:   f.Name,
			Method: f.Method,
			Flags:  f.Flags,
		}

		fw, err := zipWriter.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("failed to create file header in output zip: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file in template zip: %w", err)
		}

		if f.Name == "word/document.xml" {
			xmlContent, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read template word/document.xml: %w", err)
			}

			// Generate the table rows XML
			var rowsXML strings.Builder
			for _, row := range rows {
				rowsXML.WriteString(generateRowXML(row))
			}

			// Injects new rows right before the end of the table
			originalXML := string(xmlContent)
			replacedXML := strings.Replace(originalXML, "</w:tbl>", rowsXML.String()+"</w:tbl>", 1)

			_, err = fw.Write([]byte(replacedXML))
			if err != nil {
				return nil, fmt.Errorf("failed to write word/document.xml: %w", err)
			}
		} else {
			// Copy all other template assets directly (styles, document properties, margins, etc.)
			_, err = io.Copy(fw, rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to copy template asset '%s': %w", f.Name, err)
			}
		}
	}

	// 4. Close the zip writer
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize docx zip: %w", err)
	}

	return outputBuffer.Bytes(), nil
}

// generateRowXML constructs the OpenXML string for a single table row matching Arial landscape styles
func generateRowXML(row RoutineRow) string {
	escapedData := html.EscapeString(row.Data)
	escapedCampo := html.EscapeString(row.Campo)
	escapedExperiencia := html.EscapeString(row.Experiencia)

	// OpenXML structure for the 3-column table row
	return fmt.Sprintf(`
<w:tr>
  <w:tc>
    <w:tcPr>
      <w:tcW w:type="dxa" w:w="1417"/>
    </w:tcPr>
    <w:p>
      <w:pPr>
        <w:jc w:val="center"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
        </w:rPr>
        <w:t>%s</w:t>
      </w:r>
    </w:p>
  </w:tc>
  <w:tc>
    <w:tcPr>
      <w:tcW w:type="dxa" w:w="3969"/>
    </w:tcPr>
    <w:p>
      <w:r>
        <w:rPr>
          <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
        </w:rPr>
        <w:t>%s</w:t>
      </w:r>
    </w:p>
  </w:tc>
  <w:tc>
    <w:tcPr>
      <w:tcW w:type="dxa" w:w="9751"/>
    </w:tcPr>
    <w:p>
      <w:r>
        <w:rPr>
          <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
        </w:rPr>
        <w:t>%s</w:t>
      </w:r>
    </w:p>
  </w:tc>
</w:tr>`, escapedData, escapedCampo, escapedExperiencia)
}
