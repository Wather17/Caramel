package pdf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type pageRange struct {
	start int
	end   int
}

// SplitPDF writes one PDF per page by default, or one PDF per selected range
// when rangeSpec is non-nil. It returns the generated paths in output order.
func SplitPDF(inputPath, outputDir string, rangeSpec *string) ([]string, error) {
	inputAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
	}
	inputAbs = filepath.Clean(inputAbs)
	info, err := os.Stat(inputAbs)
	if err != nil {
		return nil, fmt.Errorf("não foi possível acessar o PDF de entrada '%s': %w", inputPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("a entrada '%s' não é um arquivo regular", inputPath)
	}

	pageCount, err := api.PageCountFile(inputAbs)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler '%s'; verifique se é um PDF válido e se não está protegido por senha: %w", inputPath, err)
	}
	if pageCount == 0 {
		return nil, fmt.Errorf("o PDF de entrada '%s' não contém páginas", inputPath)
	}

	ranges := make([]pageRange, 0, pageCount)
	if rangeSpec == nil {
		for page := 1; page <= pageCount; page++ {
			ranges = append(ranges, pageRange{start: page, end: page})
		}
	} else {
		ranges, err = parsePageRanges(*rangeSpec, pageCount)
		if err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(outputDir) == "" {
		base := strings.TrimSuffix(filepath.Base(inputAbs), filepath.Ext(inputAbs))
		outputDir = filepath.Join(filepath.Dir(inputAbs), base+"_split")
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

	base := strings.TrimSuffix(filepath.Base(inputAbs), filepath.Ext(inputAbs))
	outputPaths := make([]string, len(ranges))
	for i, pageSet := range ranges {
		outputName := splitOutputName(base, i+1, pageSet, rangeSpec == nil)
		outputPath := filepath.Join(outputAbs, outputName)
		if samePath(inputAbs, outputPath) {
			return nil, fmt.Errorf("a saída '%s' não pode substituir o PDF de entrada", outputPath)
		}
		if outputInfo, statErr := os.Stat(outputPath); statErr == nil {
			if os.SameFile(info, outputInfo) {
				return nil, fmt.Errorf("a saída '%s' aponta para o PDF de entrada", outputPath)
			}
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("não foi possível acessar a saída '%s': %w", outputPath, statErr)
		}
		outputPaths[i] = outputPath
	}

	if err := os.MkdirAll(outputAbs, 0755); err != nil {
		return nil, fmt.Errorf("não foi possível criar a pasta de saída '%s': %w", outputDir, err)
	}
	conf := model.NewDefaultConfiguration()
	if rangeSpec == nil {
		inputFile, err := os.Open(inputAbs)
		if err != nil {
			return nil, fmt.Errorf("não foi possível abrir o PDF de entrada '%s': %w", inputPath, err)
		}
		defer inputFile.Close()

		written := make([]bool, pageCount)
		err = api.ExtractPages(inputFile, nil, func(page io.Reader, pageNumber int) error {
			if pageNumber < 1 || pageNumber > len(outputPaths) {
				return fmt.Errorf("a biblioteca retornou número de página inválido: %d", pageNumber)
			}
			if err := writePagePDF(outputPaths[pageNumber-1], page); err != nil {
				return err
			}
			written[pageNumber-1] = true
			return nil
		}, conf)
		if err != nil {
			return nil, fmt.Errorf("não foi possível separar as páginas de '%s': %w", inputPath, err)
		}
		for i, ok := range written {
			if !ok {
				return nil, fmt.Errorf("a página %d não foi escrita durante a divisão do PDF", i+1)
			}
		}
		return outputPaths, nil
	}

	for i, pageSet := range ranges {
		selection := []string{strconv.Itoa(pageSet.start)}
		if pageSet.start != pageSet.end {
			selection = []string{fmt.Sprintf("%d-%d", pageSet.start, pageSet.end)}
		}
		if err := api.TrimFile(inputAbs, outputPaths[i], selection, conf); err != nil {
			return nil, fmt.Errorf("não foi possível criar a saída para %s: %w", describePageRange(pageSet), err)
		}
	}
	return outputPaths, nil
}

func writePagePDF(path string, page io.Reader) error {
	if err := pdfcpu.WriteReader(path, page); err != nil {
		return fmt.Errorf("não foi possível salvar '%s': %w", path, err)
	}
	return nil
}

func parsePageRanges(spec string, pageCount int) ([]pageRange, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, fmt.Errorf("a lista --ranges não pode ficar vazia; informe páginas como 1-3,5,7-9")
	}

	parts := strings.Split(spec, ",")
	ranges := make([]pageRange, 0, len(parts))
	selected := make(map[int]struct{})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("intervalo vazio em --ranges; use páginas como 1-3,5,7-9")
		}
		bounds := strings.Split(part, "-")
		if len(bounds) > 2 {
			return nil, fmt.Errorf("intervalo inválido '%s'; use uma página ou início-fim", part)
		}
		start, err := parsePageNumber(strings.TrimSpace(bounds[0]))
		if err != nil {
			return nil, fmt.Errorf("página inicial inválida em '%s': %w", part, err)
		}
		end := start
		if len(bounds) == 2 {
			end, err = parsePageNumber(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("página final inválida em '%s': %w", part, err)
			}
		}
		if start > end {
			return nil, fmt.Errorf("intervalo invertido '%s': início deve ser menor ou igual ao fim", part)
		}
		if start > pageCount || end > pageCount {
			return nil, fmt.Errorf("intervalo '%s' excede o total de %d páginas do PDF", part, pageCount)
		}

		for page := start; page <= end; page++ {
			if _, exists := selected[page]; exists {
				return nil, fmt.Errorf("a página %d aparece em mais de um intervalo de --ranges", page)
			}
			selected[page] = struct{}{}
		}
		ranges = append(ranges, pageRange{start: start, end: end})
	}
	return ranges, nil
}

func parsePageNumber(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("número ausente")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("'%s' não é um número de página inteiro", value)
		}
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 0, fmt.Errorf("'%s' deve ser um inteiro maior que zero", value)
	}
	return page, nil
}

func splitOutputName(base string, sequence int, pageSet pageRange, onePerPage bool) string {
	if onePerPage {
		return fmt.Sprintf("%s_page_%03d.pdf", base, pageSet.start)
	}
	if pageSet.start == pageSet.end {
		return fmt.Sprintf("%s_range_%03d_page_%03d.pdf", base, sequence, pageSet.start)
	}
	return fmt.Sprintf("%s_range_%03d_pages_%03d-%03d.pdf", base, sequence, pageSet.start, pageSet.end)
}

func describePageRange(pageSet pageRange) string {
	if pageSet.start == pageSet.end {
		return fmt.Sprintf("a página %d", pageSet.start)
	}
	return fmt.Sprintf("o intervalo %d-%d", pageSet.start, pageSet.end)
}
