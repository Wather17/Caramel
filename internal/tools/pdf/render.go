package pdf

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/tetratelabs/wazero"
)

const (
	defaultRenderDPI        = 150
	maximumRenderDPI        = 1200
	maximumRenderedPixels   = int64(20_000_000)
	wasmMemoryLimitPages    = uint32(8192) // 512 MiB
	pdfiumInstanceWaitLimit = 30 * time.Second
)

// RenderPDFPages renders every page of a PDF as a PNG or JPEG image. It stages
// all pages before replacing any destination so renderer errors never leave a
// partial set of output images.
func RenderPDFPages(inputPath, outputDir, format string, dpi int) ([]string, error) {
	format, extension, err := normalizeRenderFormat(format)
	if err != nil {
		return nil, err
	}
	if dpi < 1 || dpi > maximumRenderDPI {
		return nil, fmt.Errorf("DPI inválido %d: informe um valor entre 1 e %d", dpi, maximumRenderDPI)
	}

	inputAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
	}
	inputAbs = filepath.Clean(inputAbs)
	inputInfo, err := os.Stat(inputAbs)
	if err != nil {
		return nil, fmt.Errorf("não foi possível acessar o PDF de entrada '%s': %w", inputPath, err)
	}
	if !inputInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("a entrada '%s' não é um arquivo regular", inputPath)
	}

	if strings.TrimSpace(outputDir) == "" {
		base := strings.TrimSuffix(filepath.Base(inputAbs), filepath.Ext(inputAbs))
		outputDir = filepath.Join(filepath.Dir(inputAbs), base+"_rendered_pages")
	}
	outputAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("caminho da pasta de saída inválido '%s': %w", outputDir, err)
	}
	if outputInfo, err := os.Stat(outputAbs); err == nil && !outputInfo.IsDir() {
		return nil, fmt.Errorf("o caminho de saída '%s' existe e não é uma pasta", outputDir)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("não foi possível acessar a pasta de saída '%s': %w", outputDir, err)
	}

	stageDir, stagedPaths, outputPaths, err := renderPDFToStage(inputAbs, outputAbs, format, extension, dpi, inputInfo)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stageDir)

	if err := commitRenderedPages(stageDir, stagedPaths, outputPaths, inputAbs); err != nil {
		return nil, err
	}
	return outputPaths, nil
}

func renderPDFToStage(inputPath, outputDir, format, extension string, dpi int, inputInfo os.FileInfo) (stageDir string, stagedPaths, outputPaths []string, retErr error) {
	var cleanupStageDir string
	defer func() {
		if retErr != nil && cleanupStageDir != "" {
			retErr = errors.Join(retErr, os.RemoveAll(cleanupStageDir))
		}
	}()

	pool, err := webassembly.Init(webassembly.Config{
		MaxIdle:       1,
		MaxTotal:      1,
		FSConfig:      wazero.NewFSConfig(),
		RuntimeConfig: wazero.NewRuntimeConfig().WithMemoryLimitPages(wasmMemoryLimitPages),
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("não foi possível iniciar o renderer PDFium WebAssembly: %w", err)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("não foi possível fechar o renderer WebAssembly: %w", err))
		}
	}()

	instance, err := pool.GetInstance(pdfiumInstanceWaitLimit)
	if err != nil {
		return "", nil, nil, fmt.Errorf("não foi possível abrir uma instância do renderer PDFium: %w", err)
	}
	defer func() {
		if err := instance.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("não foi possível liberar a instância do renderer: %w", err))
		}
	}()

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("não foi possível abrir o PDF '%s': %w", inputPath, err)
	}
	defer inputFile.Close()

	document, err := instance.OpenDocument(&requests.OpenDocument{FileReader: inputFile, FileReaderSize: inputInfo.Size()})
	if err != nil {
		return "", nil, nil, fmt.Errorf("não foi possível ler '%s'; verifique se é um PDF válido e se não está protegido por senha: %w", inputPath, err)
	}
	defer func() {
		if _, err := instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document.Document}); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("não foi possível fechar o PDF de entrada: %w", err))
		}
	}()

	pageCountResponse, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: document.Document})
	if err != nil {
		return "", nil, nil, fmt.Errorf("não foi possível contar as páginas de '%s': %w", inputPath, err)
	}
	pageCount := pageCountResponse.PageCount
	if pageCount == 0 {
		return "", nil, nil, fmt.Errorf("o PDF de entrada '%s' não contém páginas", inputPath)
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	pageSizes := make([]imagePixelSize, pageCount)
	outputPaths = make([]string, pageCount)
	for pageIndex := 0; pageIndex < pageCount; pageIndex++ {
		page := requests.Page{ByIndex: &requests.PageByIndex{Document: document.Document, Index: pageIndex}}
		size, err := instance.GetPageSizeInPixels(&requests.GetPageSizeInPixels{Page: page, DPI: dpi})
		if err != nil {
			return "", nil, nil, fmt.Errorf("não foi possível obter as dimensões da página %d: %w", pageIndex+1, err)
		}
		if size.Width < 1 || size.Height < 1 || int64(size.Width) > maximumRenderedPixels/int64(size.Height) {
			return "", nil, nil, fmt.Errorf("a página %d exige %dx%d pixels em %d DPI e excede o limite seguro de %d pixels; reduza o DPI", pageIndex+1, size.Width, size.Height, dpi, maximumRenderedPixels)
		}
		pageSizes[pageIndex] = imagePixelSize{width: size.Width, height: size.Height}

		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_page_%03d.%s", base, pageIndex+1, extension))
		if samePath(inputPath, outputPath) {
			return "", nil, nil, fmt.Errorf("a imagem de saída '%s' não pode substituir o PDF de entrada", outputPath)
		}
		if outputInfo, err := os.Stat(outputPath); err == nil {
			if outputInfo.IsDir() {
				return "", nil, nil, fmt.Errorf("a saída '%s' existe e é uma pasta", outputPath)
			}
			if os.SameFile(inputInfo, outputInfo) {
				return "", nil, nil, fmt.Errorf("a imagem de saída '%s' aponta para o PDF de entrada", outputPath)
			}
		} else if !os.IsNotExist(err) {
			return "", nil, nil, fmt.Errorf("não foi possível acessar a imagem de saída '%s': %w", outputPath, err)
		}
		outputPaths[pageIndex] = outputPath
	}

	stageDir, err = os.MkdirTemp("", "caramel-pdf-render-")
	if err != nil {
		return "", nil, nil, fmt.Errorf("não foi possível preparar arquivos temporários para a renderização: %w", err)
	}
	cleanupStageDir = stageDir
	stagedPaths = make([]string, pageCount)
	for pageIndex := 0; pageIndex < pageCount; pageIndex++ {
		page := requests.Page{ByIndex: &requests.PageByIndex{Document: document.Document, Index: pageIndex}}
		rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
			Page:        page,
			DPI:         dpi,
			RenderForm:  true,
			Document:    &document.Document,
			RenderFlags: enums.FPDF_RENDER_FLAG_ANNOT,
		})
		if err != nil {
			return "", nil, nil, fmt.Errorf("falha ao renderizar a página %d de '%s': %w", pageIndex+1, inputPath, err)
		}
		if rendered.Result.RenderedImage == nil || rendered.Result.Width != pageSizes[pageIndex].width || rendered.Result.Height != pageSizes[pageIndex].height {
			rendered.Cleanup()
			return "", nil, nil, fmt.Errorf("o renderer retornou dimensões inválidas para a página %d", pageIndex+1)
		}

		stagedPath := filepath.Join(stageDir, filepath.Base(outputPaths[pageIndex]))
		if err := encodeRenderedPage(stagedPath, rendered.Result.RenderedImage, format); err != nil {
			rendered.Cleanup()
			return "", nil, nil, fmt.Errorf("não foi possível salvar a página %d renderizada: %w", pageIndex+1, err)
		}
		rendered.Cleanup()
		stagedPaths[pageIndex] = stagedPath
	}
	return stageDir, stagedPaths, outputPaths, nil
}

type imagePixelSize struct {
	width  int
	height int
}

func normalizeRenderFormat(format string) (normalized, extension string, err error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "png", "png", nil
	case "jpg", "jpeg":
		return "jpg", "jpg", nil
	default:
		return "", "", fmt.Errorf("formato inválido '%s': use png ou jpg", format)
	}
}

func encodeRenderedPage(path string, renderedImage image.Image, format string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if format == "png" {
		err = png.Encode(file, renderedImage)
	} else {
		err = jpeg.Encode(file, renderedImage, &jpeg.Options{Quality: 95})
	}
	closeErr := file.Close()
	return errors.Join(err, closeErr)
}

func commitRenderedPages(stageDir string, stagedPaths, outputPaths []string, inputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPaths[0]), 0o755); err != nil {
		return fmt.Errorf("não foi possível criar a pasta de saída '%s': %w", filepath.Dir(outputPaths[0]), err)
	}
	backupDir, err := os.MkdirTemp(filepath.Dir(outputPaths[0]), ".caramel-pdf-render-backup-")
	if err != nil {
		return fmt.Errorf("não foi possível preparar a atualização da pasta de saída: %w", err)
	}
	defer os.RemoveAll(backupDir)

	backups := make(map[string]string)
	var committed []string
	rollback := func() error {
		var rollbackErr error
		for _, path := range committed {
			rollbackErr = errors.Join(rollbackErr, os.Remove(path))
		}
		for path, backup := range backups {
			rollbackErr = errors.Join(rollbackErr, os.Rename(backup, path))
		}
		return rollbackErr
	}

	for _, outputPath := range outputPaths {
		if samePath(inputPath, outputPath) {
			return errors.Join(fmt.Errorf("a imagem de saída '%s' não pode substituir o PDF de entrada", outputPath), rollback())
		}
		info, err := os.Stat(outputPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return errors.Join(fmt.Errorf("não foi possível acessar a saída '%s': %w", outputPath, err), rollback())
		}
		if info.IsDir() {
			return errors.Join(fmt.Errorf("a saída '%s' existe e é uma pasta", outputPath), rollback())
		}
		backup := filepath.Join(backupDir, filepath.Base(outputPath))
		if err := os.Rename(outputPath, backup); err != nil {
			return errors.Join(fmt.Errorf("não foi possível preservar a saída existente '%s': %w", outputPath, err), rollback())
		}
		backups[outputPath] = backup
	}

	for i, stagedPath := range stagedPaths {
		if _, err := pdfcpu.CopyFile(stagedPath, outputPaths[i], true); err != nil {
			return errors.Join(fmt.Errorf("não foi possível publicar '%s': %w", outputPaths[i], err), rollback())
		}
		committed = append(committed, outputPaths[i])
	}
	return nil
}
