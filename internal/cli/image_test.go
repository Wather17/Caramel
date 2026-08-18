package cli

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestIsImageFile(t *testing.T) {
	valid := []string{"test.png", "TEST.JPG", "foo/bar.jpeg", "image.webp"}
	for _, f := range valid {
		if !isImageFile(f) {
			t.Errorf("esperava isImageFile(%q) == true", f)
		}
	}

	invalid := []string{"test.txt", "doc.docx", "script.sh", "archive.zip"}
	for _, f := range invalid {
		if isImageFile(f) {
			t.Errorf("esperava isImageFile(%q) == false", f)
		}
	}
}

func TestImageColorizeCmd_NoKeyConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "")

	imgPath := filepath.Join(tmpDir, "sample.png")

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)

	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("falha ao criar arquivo de imagem temporária: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("falha ao codificar PNG: %v", err)
	}
	f.Close()

	// Sem chave de API no ambiente/configuração, o comando deve falhar graciosamente
	errCmd := imageColorizeCmd.RunE(imageColorizeCmd, []string{imgPath})
	if errCmd == nil {
		t.Errorf("esperava erro por ausência de chave de API do OpenRouter")
	}
}

func TestImageColorizeCmd_Docx_NoKeyConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "")

	docxPath := filepath.Join(tmpDir, "atividade.docx")
	if err := os.WriteFile(docxPath, []byte("fake docx"), 0644); err != nil {
		t.Fatalf("falha ao criar docx temporário: %v", err)
	}

	err := imageColorizeCmd.RunE(imageColorizeCmd, []string{docxPath})
	if err == nil {
		t.Errorf("esperava erro por ausência de chave OpenRouter ao processar docx")
	}
}

func TestImageColorizeCmd_InvalidDocxExtension(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "test-key")

	txtPath := filepath.Join(tmpDir, "arquivo.txt")
	if err := os.WriteFile(txtPath, []byte("texto simples"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo txt: %v", err)
	}

	err := imageColorizeCmd.RunE(imageColorizeCmd, []string{txtPath})
	if err == nil {
		t.Errorf("esperava erro ao tentar colorir arquivo txt não-imagem")
	}
}

