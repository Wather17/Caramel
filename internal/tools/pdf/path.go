package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ResolveFuzzyPath tenta localizar um arquivo ou diretório no sistema de arquivos.
// Se a busca exata falhar, realiza uma busca insensível a maiúsculas/minúsculas,
// convertendo travessões (–, —) para hífen (-) e tratando caracteres ordinais (º, ª).
func ResolveFuzzyPath(inputPath string) (string, os.FileInfo, error) {
	// 1. Tenta a resolução exata primeiro
	stat, err := os.Stat(inputPath)
	if err == nil {
		return inputPath, stat, nil
	}

	// 2. Se falhar, tenta localizar o item no diretório pai
	dir := filepath.Dir(inputPath)
	if dir == "" {
		dir = "."
	}
	targetBase := filepath.Base(inputPath)

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return "", nil, fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
	}

	normalizedTarget := normalizePathString(targetBase)

	for _, entry := range entries {
		if normalizePathString(entry.Name()) == normalizedTarget {
			realPath := filepath.Join(dir, entry.Name())
			info, infoErr := entry.Info()
			if infoErr != nil {
				return "", nil, fmt.Errorf("erro ao obter informações de '%s': %w", realPath, infoErr)
			}
			return realPath, info, nil
		}
	}

	return "", nil, fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
}

// Clean2UpSuffix remove sufixos duplicados _2up (case-insensitive) do nome de um arquivo ou pasta
func Clean2UpSuffix(name string) string {
	cleaned := strings.TrimSpace(name)
	for {
		lower := strings.ToLower(cleaned)
		if strings.HasSuffix(lower, "_2up") {
			cleaned = strings.TrimSpace(cleaned[:len(cleaned)-4])
		} else {
			break
		}
	}
	return cleaned
}

func normalizePathString(s string) string {
	// 1. Converte travessões unicode (en-dash U+2013, em-dash U+2014) para hífen ASCII '-'
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "—", "-")

	// 2. Substitui ordinais 'º' por 'o' e 'ª' por 'a'
	s = strings.ReplaceAll(s, "º", "o")
	s = strings.ReplaceAll(s, "ª", "a")

	// 3. Remove acentos e diacríticos
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	sNorm, _, err := transform.String(t, s)
	if err != nil {
		sNorm = s
	}

	// 4. Converte para minúsculas
	sNorm = strings.ToLower(sNorm)

	// 5. Remove espaços duplicados e caracteres não alfanuméricos extras para tolerância máxima
	spaceRegex := regexp.MustCompile(`\s+`)
	sNorm = spaceRegex.ReplaceAllString(sNorm, " ")
	return strings.TrimSpace(sNorm)
}
