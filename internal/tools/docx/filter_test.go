package docx

import "testing"

func TestIsColorableFormat(t *testing.T) {
	colorable := []string{"png", "PNG", "jpg", "jpeg", "webp"}
	for _, f := range colorable {
		if !IsColorableFormat(f) {
			t.Errorf("formato '%s' deveria ser colorível", f)
		}
	}

	notColorable := []string{"emf", "wmf", "bin", "svg", "tiff", "gif", "bmp", "", "jpeg2000"}
	for _, f := range notColorable {
		if IsColorableFormat(f) {
			t.Errorf("formato '%s' não deveria ser colorível", f)
		}
	}
}