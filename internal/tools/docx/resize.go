package docx

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

// ResizeToMatch lê a imagem em colorizedPath e a redimensiona para ter exatamente as mesmas
// dimensões (largura e altura) da imagem original localizada em originalPath.
// Retorna os bytes da imagem redimensionada no mesmo formato original.
func ResizeToMatch(originalPath string, colorizedPath string) ([]byte, error) {
	// 1. Abre e decodifica a imagem original para obter suas dimensões
	origFile, err := os.Open(originalPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir imagem original: %w", err)
	}
	defer origFile.Close()

	origConfig, format, err := image.DecodeConfig(origFile)
	if err != nil {
		return nil, fmt.Errorf("falha ao obter dimensões da imagem original: %w", err)
	}

	// 2. Abre e decodifica a imagem colorida gerada pela IA
	colorizedFile, err := os.Open(colorizedPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir imagem colorida: %w", err)
	}
	defer colorizedFile.Close()

	colorizedImg, _, err := image.Decode(colorizedFile)
	if err != nil {
		return nil, fmt.Errorf("falha ao decodificar imagem colorida: %w", err)
	}

	// 3. Se as dimensões já forem idênticas, lê e retorna os bytes originais da colorizada para evitar re-encode
	if colorizedImg.Bounds().Dx() == origConfig.Width && colorizedImg.Bounds().Dy() == origConfig.Height {
		return os.ReadFile(colorizedPath)
	}

	// 4. Redimensiona a imagem usando o algoritmo BiLinear de alta fidelidade
	rect := image.Rect(0, 0, origConfig.Width, origConfig.Height)
	resizedImg := image.NewRGBA(rect)
	draw.BiLinear.Scale(resizedImg, rect, colorizedImg, colorizedImg.Bounds(), draw.Over, nil)

	// 5. Encoda a imagem redimensionada no mesmo formato original (PNG ou JPEG)
	var buf bytes.Buffer
	switch format {
	case "png":
		if err := png.Encode(&buf, resizedImg); err != nil {
			return nil, fmt.Errorf("falha ao codificar imagem redimensionada em PNG: %w", err)
		}
	case "jpeg", "jpg":
		if err := jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("falha ao codificar imagem redimensionada em JPEG: %w", err)
		}
	default:
		// Fallback para PNG caso o formato original seja SVG ou desconhecido
		if err := png.Encode(&buf, resizedImg); err != nil {
			return nil, fmt.Errorf("falha ao codificar imagem redimensionada (fallback): %w", err)
		}
	}

	return buf.Bytes(), nil
}
