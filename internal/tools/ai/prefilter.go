package ai

import (
	"fmt"
	"image"
	_ "image/jpeg" // registra o decoder JPEG para image.Decode
	_ "image/png"  // registra o decoder PNG para image.Decode
	"os"

	_ "golang.org/x/image/webp" // registra o decoder WEBP para image.Decode
)

const (
	// coloredPixelSaturationCutoff é a saturação mínima (HSV, 0-1) para um pixel ser considerado "colorido".
	// Valores abaixo disso indicam tons de cinza, preto ou branco (incluindo ruído sutil de compressão JPEG).
	coloredPixelSaturationCutoff = 0.15

	// coloredPixelRatioThreshold é a fração mínima de pixels coloridos para a imagem ser considerada já colorida.
	coloredPixelRatioThreshold = 0.05

	// maxPrefilterSamples limita a quantidade de pixels analisados para manter a checagem local instantânea.
	maxPrefilterSamples = 250000
)

// IsLikelyAlreadyColored analisa a imagem localmente (custo zero, sem chamada de API) e determina
// se ela já possui conteúdo colorido, medindo a saturação dos pixels no espaço de cor HSV.
//
// Retorna:
//   - alreadyColored: true se a imagem já parece colorida (deve ser pulada pela triagem);
//   - coloredRatio: fração de pixels com saturação acima do corte (útil para logs/debug);
//   - err: erro caso a imagem não possa ser decodificada localmente (ex: SVG, arquivo corrompido).
//
// Em caso de erro, o chamador deve adotar comportamento fail-open (seguir para a próxima etapa).
func IsLikelyAlreadyColored(imagePath string) (bool, float64, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return false, 0, fmt.Errorf("falha ao abrir imagem para análise local: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return false, 0, fmt.Errorf("falha ao decodificar imagem para análise local: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	totalPixels := width * height
	if totalPixels == 0 {
		return false, 0, fmt.Errorf("imagem com dimensões inválidas (%dx%d)", width, height)
	}

	// Calcula o passo de amostragem para nunca exceder maxPrefilterSamples
	step := 1
	if totalPixels > maxPrefilterSamples {
		step = totalPixels / maxPrefilterSamples
		if step < 1 {
			step = 1
		}
	}

	var sampled, colored int
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			r, g, b, a := img.At(x, y).RGBA()

			// Ignora pixels transparentes (ícones PNG com alpha têm RGB irrelevante sob áreas invisíveis)
			if a < 0x8000 {
				continue
			}

			sampled++
			if pixelSaturation(r, g, b) >= coloredPixelSaturationCutoff {
				colored++
			}
		}
	}

	if sampled == 0 {
		// Imagem totalmente transparente: nada a colorir, mas deixa a triagem por LLM decidir
		return false, 0, nil
	}

	ratio := float64(colored) / float64(sampled)
	return ratio >= coloredPixelRatioThreshold, ratio, nil
}

// pixelSaturation converte componentes RGB (16 bits por canal, padrão image.Color) para a saturação HSV (0-1)
func pixelSaturation(r, g, b uint32) float64 {
	rf := float64(r) / 65535.0
	gf := float64(g) / 65535.0
	bf := float64(b) / 65535.0

	maxC := rf
	if gf > maxC {
		maxC = gf
	}
	if bf > maxC {
		maxC = bf
	}

	minC := rf
	if gf < minC {
		minC = gf
	}
	if bf < minC {
		minC = bf
	}

	if maxC == 0 {
		return 0
	}
	return (maxC - minC) / maxC
}
