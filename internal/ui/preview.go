package ui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// RenderImageFileToANSI abre um arquivo de imagem do disco e o converte em string ANSI TrueColor.
func RenderImageFileToANSI(filePath string, maxCols, maxRows int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("falha ao abrir arquivo de imagem: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("falha ao decodificar imagem: %w", err)
	}

	return RenderImageToANSI(img, maxCols, maxRows), nil
}

// RenderImageToANSI converte uma imagem em miniatura formatada em ANSI usando caracteres half-block (▀).
// Cada célula do terminal (1 coluna x 1 linha) exibe 2 pixels verticais da imagem redimensionada.
func RenderImageToANSI(img image.Image, maxCols, maxRows int) string {
	if img == nil || maxCols <= 0 || maxRows <= 0 {
		return ""
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	if origWidth == 0 || origHeight == 0 {
		return ""
	}

	// Calcula proporções levando em conta que o caractere de texto no terminal tem proporção 1:2 (largura:altura)
	targetCols := maxCols
	targetRows := maxRows

	// 1 célula = 2 pixels de altura da imagem
	targetPixelHeight := targetRows * 2

	aspectOrig := float64(origWidth) / float64(origHeight)
	aspectTarget := float64(targetCols) / float64(targetPixelHeight)

	var scaledWidth, scaledHeight int
	if aspectOrig > aspectTarget {
		scaledWidth = targetCols
		scaledHeight = int(float64(targetCols) / aspectOrig)
	} else {
		scaledHeight = targetPixelHeight
		scaledWidth = int(float64(targetPixelHeight) * aspectOrig)
	}

	if scaledWidth < 1 {
		scaledWidth = 1
	}
	if scaledHeight < 1 {
		scaledHeight = 1
	}

	// Garante que scaledHeight seja par para formar os pares de meio-bloco
	if scaledHeight%2 != 0 {
		scaledHeight++
	}

	// Redimensiona a imagem usando o filtro cubic CatmullRom para máxima nitidez e preservação de detalhes
	dst := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	var sb strings.Builder
	charRows := scaledHeight / 2

	for r := 0; r < charRows; r++ {
		topY := r * 2
		bottomY := topY + 1

		for x := 0; x < scaledWidth; x++ {
			topColor := dst.At(x, topY)
			bottomColor := dst.At(x, bottomY)

			tR, tG, tB, tA := colorToRGBA(topColor)
			bR, bG, bB, bA := colorToRGBA(bottomColor)

			// Trata transparência: se ambos forem transparentes, insere espaço
			if tA < 128 && bA < 128 {
				sb.WriteString("\x1b[0m ")
				continue
			}

			// Se o topo for transparente mas o fundo não
			if tA < 128 {
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;0;0;0m▄", bR, bG, bB))
				continue
			}

			// Se o fundo for transparente mas o topo não
			if bA < 128 {
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▀", tR, tG, tB))
				continue
			}

			// Meio-bloco ▀: cor de frente (38) = pixel superior, cor de fundo (48) = pixel inferior
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tR, tG, tB, bR, bG, bB))
		}
		sb.WriteString("\x1b[0m\n")
	}

	return sb.String()
}

func colorToRGBA(c color.Color) (r, g, b, a uint8) {
	cr, cg, cb, ca := c.RGBA()
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)
}
