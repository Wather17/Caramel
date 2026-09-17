package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// MergePDFs combines PDFs in argument order and returns the total page count.
// Page boxes and orientation are preserved by pdfcpu. Encrypted inputs without
// a password are rejected before an output file is committed.
func MergePDFs(inputPaths []string, outputPath string) (int, error) {
	if len(inputPaths) < 2 {
		return 0, fmt.Errorf("informe ao menos dois arquivos PDF de entrada")
	}
	if strings.TrimSpace(outputPath) == "" {
		return 0, fmt.Errorf("informe o caminho do arquivo PDF de saída")
	}

	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return 0, fmt.Errorf("caminho de saída inválido '%s': %w", outputPath, err)
	}
	outputAbs = filepath.Clean(outputAbs)

	inputAbs := make([]string, 0, len(inputPaths))
	totalPages := 0
	for _, inputPath := range inputPaths {
		input, err := filepath.Abs(inputPath)
		if err != nil {
			return 0, fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
		}
		input = filepath.Clean(input)
		if samePath(input, outputAbs) {
			return 0, fmt.Errorf("o arquivo de saída não pode ser um dos arquivos de entrada: '%s'", inputPath)
		}

		info, err := os.Stat(input)
		if err != nil {
			return 0, fmt.Errorf("não foi possível acessar o PDF de entrada '%s': %w", inputPath, err)
		}
		if !info.Mode().IsRegular() {
			return 0, fmt.Errorf("o arquivo de entrada '%s' não é um arquivo regular", inputPath)
		}

		pages, err := api.PageCountFile(input)
		if err != nil {
			return 0, fmt.Errorf("não foi possível ler '%s'; verifique se é um PDF válido e se não está protegido por senha: %w", inputPath, err)
		}
		if pages == 0 {
			return 0, fmt.Errorf("o PDF de entrada '%s' não contém páginas", inputPath)
		}
		totalPages += pages
		inputAbs = append(inputAbs, input)
	}

	if outputInfo, err := os.Stat(outputAbs); err == nil {
		for i, input := range inputAbs {
			inputInfo, statErr := os.Stat(input)
			if statErr == nil && os.SameFile(outputInfo, inputInfo) {
				return 0, fmt.Errorf("o arquivo de saída não pode apontar para um arquivo de entrada: '%s'", inputPaths[i])
			}
		}
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("não foi possível acessar o caminho de saída '%s': %w", outputPath, err)
	}

	if err := api.MergeCreateFile(inputAbs, outputAbs, false, model.NewDefaultConfiguration()); err != nil {
		return 0, fmt.Errorf("não foi possível juntar os PDFs: %w", err)
	}
	return totalPages, nil
}

func samePath(first, second string) bool {
	if filepath.Clean(first) == filepath.Clean(second) {
		return true
	}
	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}
