package docx

import (
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// tinyWEBPBase64 é um WebP lossless válido (442 bytes)
const tinyWEBPBase64 = "UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA=="

func TestResizeToMatch_OriginalWebP(t *testing.T) {
	dir := t.TempDir()

	// Original em WEBP (formato suportado pelo Word via word/media)
	webpBytes, err := base64.StdEncoding.DecodeString(tinyWEBPBase64)
	if err != nil {
		t.Fatalf("falha ao decodificar fixture webp: %v", err)
	}
	origPath := filepath.Join(dir, "original.webp")
	if err := os.WriteFile(origPath, webpBytes, 0644); err != nil {
		t.Fatalf("falha ao criar webp original: %v", err)
	}

	// Imagem colorida em PNG de dimensões diferentes
	colorized := image.NewRGBA(image.Rect(0, 0, 40, 40))
	colorizedPath := filepath.Join(dir, "colorida.png")
	f, err := os.Create(colorizedPath)
	if err != nil {
		t.Fatalf("falha ao criar png: %v", err)
	}
	if err := png.Encode(f, colorized); err != nil {
		f.Close()
		t.Fatalf("falha ao codificar png: %v", err)
	}
	f.Close()

	outBytes, err := ResizeToMatch(origPath, colorizedPath)
	if err != nil {
		t.Fatalf("ResizeToMatch com original webp falhou: %v", err)
	}
	if len(outBytes) == 0 {
		t.Error("resize de webp retornou bytes vazios")
	}
}