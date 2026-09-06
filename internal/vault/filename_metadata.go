package vault

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// FilenameMetadata contém a classificação declarada implicitamente no nome de
// um arquivo. O parser é deliberadamente pequeno e determinístico: nomes que
// não seguem a convenção continuam sendo importados normalmente.
type FilenameMetadata struct {
	Category string
	Tags     []string
}

var filenameCategories = map[string]string{
	"atividade": "atividade",
	"at":        "atividade",
	"folha":     "folha",
	"sd":        "sequencia-didatica",
	"sequencia": "sequencia-didatica",
}

var filenameStopWords = map[string]bool{
	"a": true, "as": true, "ao": true, "aos": true,
	"com": true, "da": true, "das": true, "de": true,
	"do": true, "dos": true, "e": true, "em": true,
	"para": true, "por": true, "sem": true, "o": true,
	"os": true, "um": true, "uma": true, "uns": true,
	"umas": true,
}

func inferFilenameMetadata(name string) FilenameMetadata {
	base := filepath.Base(name)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	tokens := filenameTokens(stem)
	if len(tokens) == 0 {
		return FilenameMetadata{}
	}

	category := ""
	skip := make([]bool, len(tokens))
	for i, token := range tokens {
		canonical, ok := filenameCategories[token]
		if !ok {
			continue
		}
		skip[i] = true
		if category == "" {
			category = canonical
		}
		if token == "sequencia" && i+1 < len(tokens) && tokens[i+1] == "didatica" {
			skip[i+1] = true
		}
	}

	tags := make([]string, 0, len(tokens))
	for i, token := range tokens {
		if skip[i] || filenameStopWords[token] || isNumericToken(token) || token == "didatica" && i > 0 && skip[i-1] {
			continue
		}
		tags = append(tags, token)
	}
	sort.Strings(tags)
	return FilenameMetadata{Category: category, Tags: cleanTags(tags)}
}

func filenameTokens(value string) []string {
	value = normalizeFilenameText(value)
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func normalizeFilenameText(value string) string {
	decomposed := norm.NFD.String(strings.ToLower(value))
	var normalized strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		normalized.WriteRune(r)
	}
	return normalized.String()
}

func isNumericToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
