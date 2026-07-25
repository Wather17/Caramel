package pdf

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/phpdave11/gofpdf"

	_ "golang.org/x/image/webp"
)

// Options define as opções de configuração para a geração do PDF 2-up
type Options struct {
	DrawCutLine     bool    // Se true, desenha a linha vertical central de corte
	MarginMM        float64 // Margem de segurança externa em mm (padrão: 5mm)
	DuplicateSingle bool    // Se houver número ímpar de imagens ou apenas 1, duplica na mesma folha
}

// DefaultOptions retorna as opções padrão do gerador
func DefaultOptions() Options {
	return Options{
		DrawCutLine:     true,
		MarginMM:        5.0,
		DuplicateSingle: true,
	}
}

// Generate2UpPDF recebe uma lista de caminhos de imagens e gera um PDF A4 Paisagem com layout 2 por folha
func Generate2UpPDF(imagePaths []string, outputPath string, opts Options) error {
	if len(imagePaths) == 0 {
		return fmt.Errorf("nenhuma imagem fornecida para a geração do PDF")
	}

	// Se a margem não for definida ou for inválida, usa 5.0 mm
	if opts.MarginMM <= 0 {
		opts.MarginMM = 5.0
	}

	// Cria o documento PDF em Orientação Paisagem (Landscape), unidade em milímetros (mm), tamanho A4
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)

	// Dimensões A4 Paisagem em mm
	const (
		pageWidth  = 297.0
		pageHeight = 210.0
		midX       = pageWidth / 2.0 // 148.5 mm
	)

	// Prepara os pares de imagens por página
	pairs := prepareImagePairs(imagePaths, opts.DuplicateSingle)

	for _, pair := range pairs {
		pdf.AddPage()

		// Slot Esquerdo (Slot 1)
		if pair.Left != "" {
			err := renderImageInSlot(pdf, pair.Left, opts.MarginMM, opts.MarginMM, midX-2*opts.MarginMM, pageHeight-2*opts.MarginMM)
			if err != nil {
				return fmt.Errorf("erro ao inserir imagem no slot esquerdo (%s): %w", pair.Left, err)
			}
		}

		// Slot Direito (Slot 2)
		if pair.Right != "" {
			err := renderImageInSlot(pdf, pair.Right, midX+opts.MarginMM, opts.MarginMM, midX-2*opts.MarginMM, pageHeight-2*opts.MarginMM)
			if err != nil {
				return fmt.Errorf("erro ao inserir imagem no slot direito (%s): %w", pair.Right, err)
			}
		}

		// Desenha a linha de corte vertical no meio da página se habilitado
		if opts.DrawCutLine {
			pdf.SetDrawColor(180, 180, 180)
			pdf.SetLineWidth(0.3)
			pdf.SetDashPattern([]float64{2, 2}, 0) // Linha tracejada suave
			pdf.Line(midX, opts.MarginMM, midX, pageHeight-opts.MarginMM)
			pdf.SetDashPattern([]float64{}, 0)     // Restaura linha contínua
		}
	}

	// Garante que o diretório de destino exista
	outDir := filepath.Dir(outputPath)
	if outDir != "." && outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("falha ao criar pasta de saída: %w", err)
		}
	}

	return pdf.OutputFileAndClose(outputPath)
}

type imagePair struct {
	Left  string
	Right string
}

// prepareImagePairs agrupa a lista de imagens em pares de 2 por folha
func prepareImagePairs(imagePaths []string, duplicateSingle bool) []imagePair {
	var pairs []imagePair

	// Caso especial: apenas 1 imagem fornecida -> duplica lado a lado na mesma folha
	if len(imagePaths) == 1 {
		img := imagePaths[0]
		if duplicateSingle {
			return []imagePair{{Left: img, Right: img}}
		}
		return []imagePair{{Left: img, Right: ""}}
	}

	for i := 0; i < len(imagePaths); i += 2 {
		left := imagePaths[i]
		right := ""

		if i+1 < len(imagePaths) {
			right = imagePaths[i+1]
		} else if duplicateSingle {
			// Se sobrou 1 imagem no final e duplicateSingle é true, duplica ela
			right = left
		}

		pairs = append(pairs, imagePair{Left: left, Right: right})
	}

	return pairs
}

// renderImageInSlot calcula as dimensões mantendo a proporção (Aspect Ratio) e centraliza a imagem no slot
func renderImageInSlot(pdf *gofpdf.Fpdf, imgPath string, slotX, slotY, maxW, maxH float64) error {
	imgW, imgH, err := getImageDimensions(imgPath)
	if err != nil {
		return err
	}

	imgAspect := float64(imgW) / float64(imgH)
	slotAspect := maxW / maxH

	var renderW, renderH float64

	if imgAspect > slotAspect {
		// A imagem é mais larga que o slot -> limita pela largura
		renderW = maxW
		renderH = maxW / imgAspect
	} else {
		// A imagem é mais alta que o slot -> limita pela altura
		renderH = maxH
		renderW = maxH * imgAspect
	}

	// Centraliza a imagem dentro do slot
	offsetX := (maxW - renderW) / 2.0
	offsetY := (maxH - renderH) / 2.0

	finalX := slotX + offsetX
	finalY := slotY + offsetY

	// Detecta extensão/formato para a API do gofpdf
	ext := strings.ToUpper(filepath.Ext(imgPath))
	var imageType string
	switch ext {
	case ".PNG":
		imageType = "PNG"
	case ".JPG", ".JPEG":
		imageType = "JPG"
	default:
		imageType = ""
	}

	pdf.ImageOptions(imgPath, finalX, finalY, renderW, renderH, false, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")
	return nil
}

// getImageDimensions lê o cabeçalho da imagem para obter a resolução em pixels sem carregar toda a imagem na memória
func getImageDimensions(imgPath string) (int, int, error) {
	file, err := os.Open(imgPath)
	if err != nil {
		return 0, 0, fmt.Errorf("não foi possível abrir imagem '%s': %w", imgPath, err)
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, fmt.Errorf("falha ao ler dimensões da imagem '%s': %w", imgPath, err)
	}

	return cfg.Width, cfg.Height, nil
}
