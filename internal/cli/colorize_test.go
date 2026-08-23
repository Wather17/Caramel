package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestColorizeCmd_NonImageOrDocxFile(t *testing.T) {
	err := imageColorizeCmd.RunE(imageColorizeCmd, []string{"documento.pdf"})
	if err == nil {
		t.Errorf("esperava erro ao passar arquivo não suportado para colorize")
	}
}

func TestColorizeCmd_NoAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "")

	docxPath := filepath.Join(tmpDir, "teste.docx")
	if err := os.WriteFile(docxPath, []byte("fake docx"), 0644); err != nil {
		t.Fatalf("falha ao criar docx de teste: %v", err)
	}

	err := imageColorizeCmd.RunE(imageColorizeCmd, []string{docxPath})
	if err == nil {
		t.Errorf("esperava erro por ausência de chave de API")
	}
}

func TestRunProcessDocx_InvalidMinSize(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("OPENROUTER_API_KEY", "test-key-123")

	docxPath := filepath.Join(tmpDir, "teste.docx")
	if err := os.WriteFile(docxPath, []byte("fake docx"), 0644); err != nil {
		t.Fatalf("falha ao criar docx de teste: %v", err)
	}

	err := RunProcessDocx(ProcessDocxOptions{
		DocxPath: docxPath,
		MinSize:  "invalido-tamanho",
	})
	if err == nil {
		t.Errorf("esperava erro ao passar tamanho mínimo inválido")
	}
}
