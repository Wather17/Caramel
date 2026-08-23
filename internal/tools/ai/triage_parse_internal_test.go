package ai

import (
	"strings"
	"testing"
)

func TestParseTriageResponse(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantErr     bool
		wantShould  bool
	}{
		{name: "json puro aprovando", raw: `{"should_colorize": true, "reason": "ilustração"}`, wantShould: true},
		{name: "json puro rejeitando", raw: `{"should_colorize": false, "reason": "foto"}`, wantShould: false},
		{name: "cercado por bloco markdown", raw: "```json\n{\"should_colorize\": true, \"reason\": \"ok\"}\n```", wantShould: true},
		{name: "texto extra ao redor do json", raw: "Claro! Aqui está: {\"should_colorize\": false, \"reason\": \"tabela\"} Espero ajudar.", wantShould: false},
		{name: "sem json retorna erro", raw: "não sei o que dizer", wantErr: true},
		{name: "json malformado retorna erro", raw: `{"should_colorize": talvez}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseTriageResponse(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("esperado erro para %q, obtido %+v", tt.raw, res)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if res.ShouldColorize != tt.wantShould {
				t.Errorf("should_colorize = %v, esperado %v (reason=%q)", res.ShouldColorize, tt.wantShould, res.Reason)
			}
		})
	}
}

func TestTruncateForError(t *testing.T) {
	curto := "resposta pequena"
	if got := truncateForError(curto); got != curto {
		t.Errorf("texto curto não deveria ser truncado, obtido %q", got)
	}

	got := truncateForError(strings.Repeat("x", 300))
	if !strings.HasSuffix(got, "...") || len(got) > 210 {
		t.Errorf("texto longo deveria ser truncado a ~200 chars com reticências, obtido %d chars", len(got))
	}
}
