package compress

import (
	"bytes"
	"fmt"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
)

const MaxPixels = 100_000_000

type ImageResult struct {
	Data              []byte
	Format, Extension string
	Width, Height     int
	Skipped           bool
	Reason            string
}

func IsSupportedImageFile(path string) bool { return supportedExt(path) }
func supportedExt(path string) bool {
	e := stringsLowerExt(path)
	switch e {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".tif", ".tiff":
		return true
	}
	return false
}
func stringsLowerExt(p string) string { return strings.ToLower(filepath.Ext(p)) }
func CompressImageBytes(data []byte, opts ResolvedOptions) (ImageResult, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageResult{}, fmt.Errorf("decodificar imagem: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return ImageResult{}, fmt.Errorf("dimensões da imagem excedem o limite de segurança")
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ImageResult{}, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if opts.MaxDim > 0 && (w > opts.MaxDim || h > opts.MaxDim) {
		nw, nh := w, h
		if w >= h {
			nh = h * opts.MaxDim / w
			nw = opts.MaxDim
		} else {
			nw = w * opts.MaxDim / h
			nh = opts.MaxDim
		}
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img = dst
		w, h = nw, nh
	}
	transparent := false
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y && !transparent; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xffff {
				transparent = true
				break
			}
		}
	}
	outFormat := "jpeg"
	if transparent || (opts.KeepFormat && format == "png") {
		outFormat = "png"
	} else if opts.KeepFormat && format != "jpeg" {
		return ImageResult{Skipped: true, Reason: "formato sem encoder nativo com --keep-format"}, nil
	}
	var out bytes.Buffer
	if outFormat == "jpeg" {
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: opts.Quality})
	} else {
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		err = enc.Encode(&out, img)
	}
	if err != nil {
		return ImageResult{}, err
	}
	if len(out.Bytes()) >= len(data) {
		return ImageResult{Skipped: true, Reason: "resultado não reduziu o tamanho original", Width: w, Height: h}, nil
	}
	ext := ".jpg"
	if outFormat == "png" {
		ext = ".png"
	}
	return ImageResult{Data: out.Bytes(), Format: outFormat, Extension: ext, Width: w, Height: h}, nil
}
