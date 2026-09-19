package ai_test

import (
	"errors"
	"strings"
	"testing"

	"caramel/internal/tools/ai"
)

func TestParseStructuredArrayAceitaTextoIncidentalEValidaUnicidade(t *testing.T) {
	raw := "Aqui está o resultado:\n```json\n[{\"name\":\"Trigo\",\"prompt\":\"trigo em fundo branco\"}]\n```"
	parsed, err := ai.ParseStructuredArray(raw, "síntese de prompts", "modelo-texto")
	if err != nil {
		t.Fatalf("array válido deveria ser extraído: %v", err)
	}
	if !strings.Contains(string(parsed), `"Trigo"`) {
		t.Fatalf("array extraído inesperado: %s", parsed)
	}
}

func TestParseStructuredJSONRejeitaContratoInvalidoEAmbiguo(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		kind ai.StructuredJSONKind
		want string
	}{
		{name: "texto sem json", raw: "A lone scene description", kind: ai.StructuredJSONArray, want: "array JSON"},
		{name: "json truncado", raw: `[{"name":"Trigo"}`, kind: ai.StructuredJSONArray, want: "JSON inválido"},
		{name: "tipo errado", raw: `{"should_colorize":true}`, kind: ai.StructuredJSONArray, want: "encontrado um objeto"},
		{name: "dois valores", raw: `[{"name":"A"}] e [{"name":"B"}]`, kind: ai.StructuredJSONArray, want: "mais de um"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ai.ParseStructuredJSON(tt.raw, tt.kind, "teste", "modelo")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("erro esperado contendo %q, obtido %v", tt.want, err)
			}
			var contractErr *ai.ContractOutputError
			if !errors.As(err, &contractErr) {
				t.Fatalf("erro deveria ser ContractOutputError: %T", err)
			}
			if contractErr.Model != "modelo" || contractErr.Diagnostic() == "" {
				t.Fatalf("metadados de contrato incompletos: %+v", contractErr)
			}
			if strings.Contains(err.Error(), tt.raw) {
				t.Fatalf("erro normal não deveria despejar resposta bruta: %v", err)
			}
		})
	}
}

func TestParseStructuredObjectExigeObjeto(t *testing.T) {
	parsed, err := ai.ParseStructuredObject("prefixo {\"ok\":true} sufixo", "triagem", "modelo-visao")
	if err != nil {
		t.Fatalf("objeto válido deveria ser extraído: %v", err)
	}
	if string(parsed) != `{"ok":true}` {
		t.Fatalf("objeto extraído inesperado: %s", parsed)
	}
}
