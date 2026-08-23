package cli

import (
	"testing"
	"time"
)

func TestParseResilientDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
	}{
		{name: "ano com dois dígitos", input: "30/03/26", want: time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)},
		{name: "ano completo", input: "15/08/2026", want: time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC)},
		{name: "placeholder YY maiúsculo vira ano corrente", input: "30/03/YY", want: time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)},
		{name: "placeholder yy minúsculo vira ano corrente", input: "01/02/yy", want: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)},
		{name: "sem ano assume 2026", input: "06/04", want: time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC)},
		{name: "espaços ao redor são ignorados", input: "  06/04  ", want: time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC)},
		{name: "data inválida retorna zero", input: "banana", want: time.Time{}},
		{name: "vazio retorna zero", input: "", want: time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseResilientDate(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("parseResilientDate(%q) = %v, esperado %v", tt.input, got, tt.want)
			}
		})
	}
}
