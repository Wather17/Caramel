package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// ImageCacheOptions contains the user-visible inputs that identify one generated image.
type ImageCacheOptions struct {
	Concept     string
	Theme       string
	Style       string
	CustomStyle string
	Aspect      string
	Model       string
}

// ImageCacheKey returns a stable exact-match key for a generated image request.
func ImageCacheKey(opts ImageCacheOptions) string {
	style := opts.CustomStyle
	if strings.TrimSpace(style) == "" {
		style = opts.Style
	}
	if strings.TrimSpace(style) == "" {
		style = "clipart"
	}
	aspect := opts.Aspect
	if strings.TrimSpace(aspect) == "" {
		aspect = "1:1"
	}
	model := opts.Model
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}
	canonical := struct {
		Concept string `json:"concept"`
		Theme   string `json:"theme"`
		Style   string `json:"style"`
		Aspect  string `json:"aspect"`
		Model   string `json:"model"`
	}{
		Concept: normalizeImageCacheText(opts.Concept),
		Theme:   normalizeImageCacheText(opts.Theme),
		Style:   normalizeImageCacheText(style),
		Aspect:  normalizeImageCacheText(aspect),
		Model:   normalizeImageCacheText(model),
	}
	data, _ := json.Marshal(canonical)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func normalizeImageCacheText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
