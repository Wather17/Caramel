package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/phpdave11/gofpdf"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Options define as opções de configuração para a geração do PDF 2-up
type Options struct {
	DrawCutLine     bool    // Se true, desenha a linha vertical central de corte
	MarginMM        float64 // Margem de segurança externa em mm (padrão: 5mm)
	DuplicateSingle bool    // Se houver número ímpar de imagens ou apenas 1, duplica na mesma folha
	AutoRotate      bool    // Se true, calcula e rotaciona imagens landscape quando benéfico
	RotateThreshold float64 // Porcentagem mínima de ganho de área para disparar a rotação (padrão: 15.0%)
	FitMode         string  // Modo de encaixe: "contain" (padrão) ou "cover"
	Optimize        bool    // Se true, faz downsampling e recompressão JPEG em memória das imagens pesadas
	MaxDPI          int     // Limite máximo de resolução em DPI (padrão: 300)
	Quality         int     // Qualidade de compressão JPEG de 1 a 100 (padrão: 85)
}

// DefaultOptions retorna as opções padrão do gerador
func DefaultOptions() Options {
	return Options{
		DrawCutLine:     true,
		MarginMM:        5.0,
		DuplicateSingle: true,
		AutoRotate:      true,
		RotateThreshold: 15.0,
		FitMode:         "contain",
		Optimize:        true,
		MaxDPI:          300,
		Quality:         85,
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
	if opts.RotateThreshold <= 0 {
		opts.RotateThreshold = 15.0
	}
	if opts.FitMode == "" {
		opts.FitMode = "contain"
	}
	if opts.MaxDPI <= 0 {
		opts.MaxDPI = 300
	}
	if opts.Quality <= 0 || opts.Quality > 100 {
		opts.Quality = 85
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
			err := renderImageInSlot(pdf, pair.Left, opts.MarginMM, opts.MarginMM, midX-2*opts.MarginMM, pageHeight-2*opts.MarginMM, opts)
			if err != nil {
				return fmt.Errorf("erro ao inserir imagem no slot esquerdo (%s): %w", pair.Left, err)
			}
		}

		// Slot Direito (Slot 2)
		if pair.Right != "" {
			err := renderImageInSlot(pdf, pair.Right, midX+opts.MarginMM, opts.MarginMM, midX-2*opts.MarginMM, pageHeight-2*opts.MarginMM, opts)
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

// renderImageInSlot calcula as dimensões, decide por rotação, otimiza se ativado e posiciona a imagem no slot
func renderImageInSlot(pdf *gofpdf.Fpdf, imgPath string, slotX, slotY, maxW, maxH float64, opts Options) error {
	imgW, imgH, err := getImageDimensions(imgPath)
	if err != nil {
		return err
	}

	if imgW <= 0 || imgH <= 0 {
		return fmt.Errorf("dimensões inválidas para a imagem '%s': %dx%d", imgPath, imgW, imgH)
	}

	w := float64(imgW)
	h := float64(imgH)

	// Cálculo da área normal (sem rotação)
	renderWNorm, renderHNorm := calculateFitDimensions(w, h, maxW, maxH, opts.FitMode)
	areaNorm := renderWNorm * renderHNorm

	// Cálculo da área se rotacionada em 90°
	renderWRot, renderHRot := calculateFitDimensions(h, w, maxW, maxH, opts.FitMode)
	areaRot := renderWRot * renderHRot

	shouldRotate := false
	if opts.AutoRotate && areaNorm > 0 {
		gainPercent := ((areaRot - areaNorm) / areaNorm) * 100.0
		if gainPercent >= opts.RotateThreshold {
			shouldRotate = true
		}
	}

	imageTarget := imgPath
	var imageType string

	// Otimização em memória se habilitada
	if opts.Optimize {
		reader, optFormat, optErr := optimizeImageInMemory(imgPath, maxW, maxH, opts)
		if optErr == nil && reader != nil {
			imageKey := fmt.Sprintf("opt_%s", filepath.Base(imgPath))
			imageType = optFormat
			pdf.RegisterImageOptionsReader(imageKey, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, reader)
			imageTarget = imageKey
		}
	}

	if imageType == "" {
		ext := strings.ToUpper(filepath.Ext(imgPath))
		switch ext {
		case ".PNG":
			imageType = "PNG"
		case ".JPG", ".JPEG":
			imageType = "JPG"
		default:
			imageType = ""
		}
	}

	centerX := slotX + maxW/2.0
	centerY := slotY + maxH/2.0

	if shouldRotate {
		imgDrawX := centerX - renderWRot/2.0
		imgDrawY := centerY - renderHRot/2.0

		pdf.TransformBegin()
		pdf.TransformRotate(90, centerX, centerY)
		pdf.ImageOptions(imageTarget, imgDrawX, imgDrawY, renderWRot, renderHRot, false, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")
		pdf.TransformEnd()
	} else {
		finalX := centerX - renderWNorm/2.0
		finalY := centerY - renderHNorm/2.0
		pdf.ImageOptions(imageTarget, finalX, finalY, renderWNorm, renderHNorm, false, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")
	}

	return nil
}

func optimizeImageInMemory(imgPath string, maxWMM, maxHMM float64, opts Options) (io.Reader, string, error) {
	file, err := os.Open(imgPath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao decodificar imagem '%s': %w", imgPath, err)
	}

	bounds := srcImg.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW <= 0 || srcH <= 0 {
		return nil, "", fmt.Errorf("dimensões inválidas na imagem '%s'", imgPath)
	}

	maxDPI := opts.MaxDPI
	if maxDPI <= 0 {
		maxDPI = 300
	}
	quality := opts.Quality
	if quality <= 0 || quality > 100 {
		quality = 85
	}

	// Resolução limite em pixels para 300 DPI no slot A5
	maxAllowedW := int((maxWMM / 25.4) * float64(maxDPI))
	maxAllowedH := int((maxHMM / 25.4) * float64(maxDPI))

	if maxAllowedW < maxAllowedH {
		maxAllowedW, maxAllowedH = maxAllowedH, maxAllowedW
	}

	needsRescale := srcW > maxAllowedW || srcH > maxAllowedH
	var finalImg image.Image = srcImg

	if needsRescale {
		scaleW := float64(maxAllowedW) / float64(srcW)
		scaleH := float64(maxAllowedH) / float64(srcH)
		scale := scaleW
		if scaleH < scaleW {
			scale = scaleH
		}

		newW := int(float64(srcW) * scale)
		newH := int(float64(srcH) * scale)
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}

		dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.BiLinear.Scale(dst, dst.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)
		finalImg = dst
	}

	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, finalImg, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, "", fmt.Errorf("falha ao comprimir imagem em memória: %w", err)
	}

	return buf, "JPG", nil
}

func calculateFitDimensions(imgW, imgH, maxW, maxH float64, fitMode string) (float64, float64) {
	imgAspect := imgW / imgH
	slotAspect := maxW / maxH

	var renderW, renderH float64

	if strings.ToLower(fitMode) == "cover" {
		if imgAspect > slotAspect {
			renderH = maxH
			renderW = maxH * imgAspect
		} else {
			renderW = maxW
			renderH = maxW / imgAspect
		}
	} else {
		// "contain" por padrão
		if imgAspect > slotAspect {
			renderW = maxW
			renderH = maxW / imgAspect
		} else {
			renderH = maxH
			renderW = maxH * imgAspect
		}
	}

	return renderW, renderH
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

// SortNatural realiza ordenação alfanumérica natural em um slice de caminhos de arquivo
func SortNatural(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		return NaturalLess(paths[i], paths[j])
	})
}

// NaturalLess compara duas strings considerando sequências numéricas (ex: "img2" < "img10")
func NaturalLess(a, b string) bool {
	chunksA := splitIntoChunks(a)
	chunksB := splitIntoChunks(b)

	minLen := len(chunksA)
	if len(chunksB) < minLen {
		minLen = len(chunksB)
	}

	for i := 0; i < minLen; i++ {
		cA := chunksA[i]
		cB := chunksB[i]

		isNumA := isDigit(cA)
		isNumB := isDigit(cB)

		if isNumA && isNumB {
			numA, errA := strconv.ParseUint(cA, 10, 64)
			numB, errB := strconv.ParseUint(cB, 10, 64)
			if errA == nil && errB == nil {
				if numA != numB {
					return numA < numB
				}
			}
		}

		if cA != cB {
			return strings.ToLower(cA) < strings.ToLower(cB)
		}
	}

	return len(chunksA) < len(chunksB)
}

func splitIntoChunks(s string) []string {
	var chunks []string
	var current strings.Builder
	var lastIsDigit bool

	for i, r := range s {
		digit := unicode.IsDigit(r)
		if i > 0 && digit != lastIsDigit {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteRune(r)
		lastIsDigit = digit
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func isDigit(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
