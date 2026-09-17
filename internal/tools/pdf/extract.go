package pdf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ExtractEmbeddedImages extracts image resources from every page and returns
// their output paths and any warnings for resources pdfcpu could not decode.
// Files are staged first and published without replacing existing files.
func ExtractEmbeddedImages(inputPath, outputDir string) ([]string, []string, error) {
	inputAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
	}
	inputAbs = filepath.Clean(inputAbs)
	inputInfo, err := os.Stat(inputAbs)
	if err != nil {
		return nil, nil, fmt.Errorf("não foi possível acessar o PDF de entrada '%s': %w", inputPath, err)
	}
	if !inputInfo.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("a entrada '%s' não é um arquivo regular", inputPath)
	}

	if strings.TrimSpace(outputDir) == "" {
		base := strings.TrimSuffix(filepath.Base(inputAbs), filepath.Ext(inputAbs))
		outputDir = filepath.Join(filepath.Dir(inputAbs), base+"_embedded_images")
	}
	outputAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, nil, fmt.Errorf("caminho da pasta de saída inválido '%s': %w", outputDir, err)
	}
	if info, err := os.Stat(outputAbs); err == nil && !info.IsDir() {
		return nil, nil, fmt.Errorf("o caminho de saída '%s' existe e não é uma pasta", outputDir)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("não foi possível acessar a pasta de saída '%s': %w", outputDir, err)
	}

	stageDir, err := os.MkdirTemp("", "caramel-pdf-images-")
	if err != nil {
		return nil, nil, fmt.Errorf("não foi possível preparar arquivos temporários para a extração: %w", err)
	}
	defer os.RemoveAll(stageDir)

	base := strings.TrimSuffix(filepath.Base(inputAbs), filepath.Ext(inputAbs))
	imageIndexByPage := make(map[int]int)
	var stagedPaths, outputPaths, warnings []string
	conf := model.NewDefaultConfiguration()
	conf.UnsupportedResourcePolicy = model.UnsupportedResourceSkip

	input, err := os.Open(inputAbs)
	if err != nil {
		return nil, nil, fmt.Errorf("não foi possível abrir o PDF '%s': %w", inputPath, err)
	}
	extractErr := api.ExtractImages(input, nil, func(img model.Image, _ bool, _ int) error {
		if img.PageNr < 1 {
			return fmt.Errorf("pdfcpu retornou imagem sem número de página válido")
		}
		imageIndexByPage[img.PageNr]++
		imageIndex := imageIndexByPage[img.PageNr]
		extension, supported := extractedImageExtension(img.FileType)
		if !supported {
			warnings = append(warnings, fmt.Sprintf("página %d, recurso %d: formato emitido pelo extrator '%s' não é suportado e foi ignorado", img.PageNr, imageIndex, img.FileType))
			return nil
		}
		if img.Reader == nil {
			warnings = append(warnings, fmt.Sprintf("página %d, recurso %d: o extrator não retornou dados e o recurso foi ignorado", img.PageNr, imageIndex))
			return nil
		}

		name := fmt.Sprintf("%s_page_%03d_image_%03d.%s", base, img.PageNr, imageIndex, extension)
		stagedPath := filepath.Join(stageDir, name)
		if err := pdfcpu.WriteReader(stagedPath, img.Reader); err != nil {
			return fmt.Errorf("não foi possível preparar a imagem da página %d, recurso %d: %w", img.PageNr, imageIndex, err)
		}
		stagedPaths = append(stagedPaths, stagedPath)
		outputPaths = append(outputPaths, filepath.Join(outputAbs, name))
		return nil
	}, conf)
	closeErr := input.Close()
	if extractErr != nil {
		var unsupportedErr *api.UnsupportedResourceError
		if errors.As(extractErr, &unsupportedErr) {
			warnings = append(warnings, "alguns recursos de imagem não puderam ser decodificados: "+extractErr.Error())
		} else {
			return nil, nil, fmt.Errorf("não foi possível extrair imagens de '%s': %w", inputPath, errors.Join(extractErr, closeErr))
		}
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("não foi possível fechar o PDF de entrada '%s': %w", inputPath, closeErr)
	}
	if len(stagedPaths) == 0 {
		return nil, warnings, nil
	}
	if err := publishExtractedImages(stagedPaths, outputPaths); err != nil {
		return nil, nil, err
	}
	return outputPaths, warnings, nil
}

func extractedImageExtension(fileType string) (string, bool) {
	extension := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fileType), "."))
	switch extension {
	case "jpg", "png", "tif", "jpx", "jbig2":
		return extension, true
	default:
		return "", false
	}
}

func publishExtractedImages(stagedPaths, outputPaths []string) error {
	if len(stagedPaths) != len(outputPaths) {
		return fmt.Errorf("falha interna: quantidade de imagens preparadas difere da quantidade de saídas")
	}
	if len(outputPaths) == 0 {
		return nil
	}

	outputDir := filepath.Dir(outputPaths[0])
	outputDirExists := false
	if info, err := os.Stat(outputDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("o caminho de saída '%s' existe e não é uma pasta", outputDir)
		}
		outputDirExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("não foi possível acessar a pasta de saída '%s': %w", outputDir, err)
	}

	for _, outputPath := range outputPaths {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("a saída '%s' já existe; nenhuma imagem foi sobrescrita", outputPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("não foi possível acessar a saída '%s': %w", outputPath, err)
		}
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("não foi possível criar a pasta de saída '%s': %w", outputDir, err)
	}
	var committed []string
	rollback := func(cause error) error {
		var rollbackErr error
		for i := len(committed) - 1; i >= 0; i-- {
			rollbackErr = errors.Join(rollbackErr, os.Remove(committed[i]))
		}
		if !outputDirExists {
			rollbackErr = errors.Join(rollbackErr, os.Remove(outputDir))
		}
		return errors.Join(cause, rollbackErr)
	}

	for i, stagedPath := range stagedPaths {
		created, err := pdfcpu.CopyFile(stagedPath, outputPaths[i], false)
		if err != nil {
			return rollback(fmt.Errorf("não foi possível publicar '%s': %w", outputPaths[i], err))
		}
		if !created {
			return rollback(fmt.Errorf("a saída '%s' já existe; nenhuma imagem foi sobrescrita", outputPaths[i]))
		}
		committed = append(committed, outputPaths[i])
	}
	return nil
}
