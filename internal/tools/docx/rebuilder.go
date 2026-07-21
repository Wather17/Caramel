package docx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
)

// RebuildDocx copia a estrutura de originalDocxPath para targetDocxPath,
// substituindo os arquivos internos cujos caminhos constem no mapa de replacements pelos novos bytes fornecidos.
func RebuildDocx(originalDocxPath, targetDocxPath string, replacements map[string][]byte) error {
	// 1. Abre o zip/docx original
	r, err := zip.OpenReader(originalDocxPath)
	if err != nil {
		return fmt.Errorf("falha ao abrir docx original para reconstrução: %w", err)
	}
	defer r.Close()

	// 2. Cria o arquivo de destino docx
	outFile, err := os.Create(targetDocxPath)
	if err != nil {
		return fmt.Errorf("falha ao criar docx de destino: %w", err)
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	// 3. Itera por todos os arquivos originais
	for _, f := range r.File {
		// Cria o cabeçalho idêntico no novo zip
		fw, err := w.CreateHeader(&zip.FileHeader{
			Name:   f.Name,
			Method: f.Method,
			Flags:  f.Flags,
		})
		if err != nil {
			return fmt.Errorf("erro ao criar arquivo '%s' no novo zip: %w", f.Name, err)
		}

		// Se o arquivo tiver uma substituição de imagem declarada, grava os bytes da imagem colorida/redimensionada
		if newBytes, found := replacements[f.Name]; found {
			if _, err := fw.Write(newBytes); err != nil {
				return fmt.Errorf("erro ao escrever dados de substituição para '%s': %w", f.Name, err)
			}
			continue
		}

		// Caso contrário, copia os bytes originais diretamente
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("erro ao abrir arquivo original '%s': %w", f.Name, err)
		}

		if _, err := io.Copy(fw, rc); err != nil {
			rc.Close()
			return fmt.Errorf("erro ao copiar conteúdo original de '%s': %w", f.Name, err)
		}
		rc.Close()
	}

	return nil
}
