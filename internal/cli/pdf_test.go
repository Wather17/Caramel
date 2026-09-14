package cli

import (
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func writePDFCLIImageFixture(t *testing.T, path, format string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 24, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 130, B: 220, A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("falha ao criar fixture %s: %v", path, err)
	}
	defer f.Close()

	switch format {
	case "jpeg":
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 100})
	case "gif":
		err = gif.Encode(f, img, nil)
	case "bmp":
		err = bmp.Encode(f, img)
	case "tiff":
		err = tiff.Encode(f, img, nil)
	default:
		t.Fatalf("formato de fixture não suportado: %s", format)
	}
	if err != nil {
		t.Fatalf("falha ao codificar fixture %s: %v", path, err)
	}
}

func configurePDF2UpTestFlags() {
	pdfOutputDir = ""
	pdfDrawCutLine = true
	pdfMarginMM = 5
	pdfDuplicateSingle = true
	pdfAutoRotate = true
	pdfRotateThreshold = 15
	pdfFitMode = "contain"
	pdfSize = "large"
	pdfOptimize = false
	pdfMaxDPI = 300
	pdfQuality = 85
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}

func TestPDF2UpCmd_ArquivoJFIFMaiusculo(t *testing.T) {
	configurePDF2UpTestFlags()
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "atividade.JFIF")
	writePDFCLIImageFixture(t, imgPath, "jpeg")

	if err := pdf2UpCmd.RunE(pdf2UpCmd, []string{imgPath}); err != nil {
		t.Fatalf("2up deveria aceitar JFIF: %v", err)
	}

	outPath := filepath.Join(tempDir, "atividade_2up.pdf")
	stat, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("PDF do arquivo JFIF não foi criado: %v", err)
	}
	if stat.Size() == 0 {
		t.Fatal("PDF do arquivo JFIF está vazio")
	}
}

func TestPDF2UpCmd_PastaComFormatosMistos(t *testing.T) {
	configurePDF2UpTestFlags()
	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "atividades")
	if err := os.Mkdir(inputDir, 0755); err != nil {
		t.Fatalf("falha ao criar pasta de fixtures: %v", err)
	}

	writePDFCLIImageFixture(t, filepath.Join(inputDir, "01.JIF"), "jpeg")
	writePDFCLIImageFixture(t, filepath.Join(inputDir, "02.GIF"), "gif")
	writePDFCLIImageFixture(t, filepath.Join(inputDir, "03.BMP"), "bmp")
	writePDFCLIImageFixture(t, filepath.Join(inputDir, "04.TIFF"), "tiff")
	if err := os.WriteFile(filepath.Join(inputDir, "leia-me.txt"), []byte("ignorar"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo não suportado: %v", err)
	}

	if err := pdf2UpCmd.RunE(pdf2UpCmd, []string{inputDir}); err != nil {
		t.Fatalf("2up deveria processar pasta mista: %v", err)
	}

	outPath := filepath.Join(inputDir, "atividades_2up.pdf")
	stat, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("PDF da pasta mista não foi criado: %v", err)
	}
	if stat.Size() == 0 {
		t.Fatal("PDF da pasta mista está vazio")
	}
}

func TestPDF2UpCmd_ExtensaoAceitaComConteudoInvalido(t *testing.T) {
	configurePDF2UpTestFlags()
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "invalida.TIFF")
	if err := os.WriteFile(imgPath, []byte("não é uma imagem"), 0644); err != nil {
		t.Fatalf("falha ao criar entrada inválida: %v", err)
	}

	err := pdf2UpCmd.RunE(pdf2UpCmd, []string{imgPath})
	if err == nil {
		t.Fatal("conteúdo inválido com extensão aceita deveria falhar")
	}
	if !strings.Contains(err.Error(), "falha ao gerar PDF 2-up") || !strings.Contains(err.Error(), "falha ao ler dimensões") {
		t.Fatalf("erro deveria preservar a falha de decodificação, obtido: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tempDir, "invalida_2up.pdf")); !os.IsNotExist(statErr) {
		t.Fatalf("PDF não deveria ser criado para entrada inválida, stat err=%v", statErr)
	}
}

func TestPDF2UpCmd_ExtensaoNaoSuportadaContinuaRejeitada(t *testing.T) {
	configurePDF2UpTestFlags()
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "atividade.svg")
	if err := os.WriteFile(imgPath, []byte("<svg></svg>"), 0644); err != nil {
		t.Fatalf("falha ao criar entrada SVG: %v", err)
	}

	err := pdf2UpCmd.RunE(pdf2UpCmd, []string{imgPath})
	if err == nil {
		t.Fatal("SVG deveria continuar sendo rejeitado")
	}
	if !strings.Contains(err.Error(), "JPG/JPEG/JPE/JFIF/JIF") {
		t.Fatalf("erro deveria listar as extensões atuais, obtido: %v", err)
	}
}
