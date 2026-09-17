package cli

import (
	"bytes"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFPagesRenderCommandUsesFormatDPIAndOutputDirectory(t *testing.T) {
	configurePDFPagesTestFlags(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "sample.pdf")
	writePDFSplitFixture(t, input, 1)
	outputDir := filepath.Join(dir, "rendered")
	if err := pdfPagesRenderCmd.Flags().Set("format", "jpg"); err != nil {
		t.Fatalf("não foi possível configurar --format: %v", err)
	}
	if err := pdfPagesRenderCmd.Flags().Set("dpi", "72"); err != nil {
		t.Fatalf("não foi possível configurar --dpi: %v", err)
	}
	if err := pdfPagesRenderCmd.Flags().Set("output-dir", outputDir); err != nil {
		t.Fatalf("não foi possível configurar --output-dir: %v", err)
	}

	oldOut := pdfPagesRenderCmd.OutOrStdout()
	var stdout bytes.Buffer
	pdfPagesRenderCmd.SetOut(&stdout)
	defer pdfPagesRenderCmd.SetOut(oldOut)
	if err := pdfPagesRenderCmd.RunE(pdfPagesRenderCmd, []string{input}); err != nil {
		t.Fatalf("pdf pages render falhou: %v", err)
	}

	output := filepath.Join(outputDir, "sample_page_001.jpg")
	if !strings.Contains(stdout.String(), output) || !strings.Contains(stdout.String(), "1 imagem(ns)") {
		t.Errorf("resultado da CLI não informa saída e quantidade: %q", stdout.String())
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatalf("imagem JPEG não foi criada: %v", err)
	}
	config, err := jpeg.DecodeConfig(file)
	_ = file.Close()
	if err != nil {
		t.Fatalf("imagem JPEG não pode ser decodificada: %v", err)
	}
	if config.Width < 1 || config.Height < 1 {
		t.Fatalf("dimensões JPEG inválidas: %dx%d", config.Width, config.Height)
	}
}

func configurePDFPagesTestFlags(t *testing.T) {
	t.Helper()
	pdfPagesFormat = "png"
	pdfPagesDPI = 150
	pdfPagesOutputDir = ""
	pdfPagesRenderCmd.Flags().Lookup("format").Changed = false
	pdfPagesRenderCmd.Flags().Lookup("dpi").Changed = false
	pdfPagesRenderCmd.Flags().Lookup("output-dir").Changed = false
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}
