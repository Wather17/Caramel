package docx

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)
	multiSpace       = regexp.MustCompile(`\s+`)
)

// SanitizeFolderName gera um nome de pasta seguro, limpo e compacto (usando espaços) a partir do nome de um arquivo .docx
func SanitizeFolderName(docxPath string) string {
	// Extrai apenas o nome do arquivo sem caminho e sem a extensão .docx
	base := filepath.Base(docxPath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// 1. Remove acentos e diacríticos (ex: HISTÓRIA -> HISTORIA, SEÇÃO -> SECAO)
	cleaned := removeAccents(nameWithoutExt)

	// 2. Converte para minúsculas
	cleaned = strings.ToLower(cleaned)

	// 3. Substitui caracteres especiais e pontuações por espaço ' '
	cleaned = nonAlphaNumRegex.ReplaceAllString(cleaned, " ")

	// 4. Remove espaços duplicados
	cleaned = multiSpace.ReplaceAllString(cleaned, " ")

	// 5. Remove espaços no início e no fim
	cleaned = strings.TrimSpace(cleaned)

	// 6. Limita o tamanho máximo do nome (máximo 45 caracteres)
	const maxLength = 45
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
		cleaned = strings.TrimSpace(cleaned)
	}

	// 7. Caso o nome fique vazio após a limpeza, usa um fallback genérico
	if cleaned == "" {
		cleaned = "doc"
	}

	return "imagens " + cleaned
}

// removeAccents remove acentos de caracteres Unicode (ex: 'á' -> 'a')
func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return result
}
