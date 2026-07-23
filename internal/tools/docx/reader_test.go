package docx

import (
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
