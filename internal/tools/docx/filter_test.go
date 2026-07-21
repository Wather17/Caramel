package docx_test

import (
	"testing"

	"caramel/internal/tools/docx"
)

func TestParseSizeInBytes(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"20KB", 20480, false},
		{"20kb", 20480, false},
		{"1MB", 1048576, false},
		{"500B", 500, false},
		{"100", 100, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		bytes, err := docx.ParseSizeInBytes(tt.input)
		if (err != nil) != tt.hasError {
			t.Errorf("Para entrada %q: esperado erro %v, obtido %v", tt.input, tt.hasError, err)
		}
		if err == nil && bytes != tt.expected {
			t.Errorf("Para entrada %q: esperado %d bytes, obtido %d", tt.input, tt.expected, bytes)
		}
	}
}

func TestFilterImagesByMinSize(t *testing.T) {
	images := []docx.ExtractedImage{
		{OriginalName: "logo.png", Size: 10 * 1024},     // 10 KB
		{OriginalName: "exercicio.png", Size: 200 * 1024}, // 200 KB
	}

	minSize := int64(20 * 1024) // 20 KB
	kept, skipped := docx.FilterImagesByMinSize(images, minSize)

	if len(kept) != 1 || kept[0].OriginalName != "exercicio.png" {
		t.Errorf("Esperado manter apenas 'exercicio.png', mantido: %v", kept)
	}

	if len(skipped) != 1 || skipped[0].OriginalName != "logo.png" {
		t.Errorf("Esperado ignorar apenas 'logo.png', ignorados: %v", skipped)
	}
}
