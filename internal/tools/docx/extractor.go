package docx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractedImage representa uma imagem encontrada no arquivo .docx
type ExtractedImage struct {
	OriginalName string // ex: image1.png
	PathInZip    string // ex: word/media/image1.png
	Size         int64  // tamanho em bytes
	Format       string // extensão ex: png, jpeg, svg
}

// ExtractionResult contém o resumo do processo de extração
type ExtractionResult struct {
	DocxPath       string
	OutputDir      string
	TotalExtracted int
	Images         []ExtractedImage
}

// ListImages inspeciona um arquivo .docx e retorna a lista de imagens contidas nele sem extraí-las
func ListImages(docxPath string) ([]ExtractedImage, error) {
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir arquivo .docx (verifique se é um arquivo válido): %w", err)
	}
	defer r.Close()

	var images []ExtractedImage

	for _, f := range r.File {
		// As imagens de um arquivo .docx normalmente ficam na pasta word/media/
		if strings.HasPrefix(f.Name, "word/media/") && !f.FileInfo().IsDir() {
			filename := filepath.Base(f.Name)
			ext := strings.TrimPrefix(filepath.Ext(filename), ".")

			images = append(images, ExtractedImage{
				OriginalName: filename,
				PathInZip:    f.Name,
				Size:         f.FileInfo().Size(),
				Format:       strings.ToLower(ext),
			})
		}
	}

	return images, nil
}

// ExtractImages extrai todas as imagens do arquivo .docx para o diretório de destino (outputDir)
func ExtractImages(docxPath string, outputDir string) (*ExtractionResult, error) {
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir arquivo .docx: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar diretório de saída '%s': %w", outputDir, err)
	}

	result := &ExtractionResult{
		DocxPath:  docxPath,
		OutputDir: outputDir,
		Images:    make([]ExtractedImage, 0),
	}

	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "word/media/") && !f.FileInfo().IsDir() {
			filename := filepath.Base(f.Name)
			destPath := filepath.Join(outputDir, filename)

			if err := extractZipFile(f, destPath); err != nil {
				return nil, fmt.Errorf("erro ao extrair imagem '%s': %w", filename, err)
			}

			ext := strings.TrimPrefix(filepath.Ext(filename), ".")
			imgInfo := ExtractedImage{
				OriginalName: filename,
				PathInZip:    f.Name,
				Size:         f.FileInfo().Size(),
				Format:       strings.ToLower(ext),
			}

			result.Images = append(result.Images, imgInfo)
			result.TotalExtracted++
		}
	}

	return result, nil
}

// extractZipFile copia o conteúdo de um arquivo zip para o disco de destino
func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}
