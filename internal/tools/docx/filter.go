package docx

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSizeInBytes converte strings como "20KB", "1MB", "500B" ou "20" em bytes inteiros
func ParseSizeInBytes(sizeStr string) (int64, error) {
	s := strings.TrimSpace(strings.ToUpper(sizeStr))
	if s == "" {
		return 0, nil
	}

	multiplier := int64(1)
	if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	} else if strings.HasSuffix(s, "K") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "K")
	}

	s = strings.TrimSpace(s)
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("tamanho inválido '%s': informe valores como '20KB', '1MB' ou '500'", sizeStr)
	}

	return val * multiplier, nil
}

// FilterImagesByMinSize filtra uma lista de imagens mantendo apenas as que possuem tamanho >= minSizeBytes
func FilterImagesByMinSize(images []ExtractedImage, minSizeBytes int64) (kept []ExtractedImage, skipped []ExtractedImage) {
	for _, img := range images {
		if img.Size >= minSizeBytes {
			kept = append(kept, img)
		} else {
			skipped = append(skipped, img)
		}
	}
	return kept, skipped
}
