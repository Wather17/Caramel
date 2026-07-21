package docx_test

import (
	"testing"

	"caramel/internal/tools/docx"
)

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "atividade.docx",
			expected: "imagens atividade",
		},
		{
			input:    "PROVA DE HISTÓRIA 8º ANO (1º SEMESTRE) - CÓPIA (1).docx",
			expected: "imagens prova de historia 8 ano 1 semestre copia 1",
		},
		{
			input:    "PROVA DE HISTÓRIA DO BRASIL E AMÉRICA LATINA COM NOME EXTREMAMENTE ENORME E LONGO DEMAIS PARA QUALQUER PASTA.docx",
			expected: "imagens prova de historia do brasil e america latina",
		},
		{
			input:    "---!!!***??.docx",
			expected: "imagens doc",
		},
		{
			input:    "/caminho/para/meu_simulado_2026.docx",
			expected: "imagens meu simulado 2026",
		},
	}

	for _, tt := range tests {
		result := docx.SanitizeFolderName(tt.input)
		if result != tt.expected {
			t.Errorf("Para entrada %q:\n  esperado: %q\n  obtido:   %q", tt.input, tt.expected, result)
		}
	}
}
