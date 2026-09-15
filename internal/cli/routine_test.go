package cli

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"caramel/internal/tools/ai"
)

func TestParseResilientDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
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

func writeRoutineDocx(t *testing.T, path, text string) {
	t.Helper()
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	entry, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatalf("falha ao criar document.xml: %v", err)
	}
	_, _ = fmt.Fprintf(entry, `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>%s</w:t></w:r></w:p></w:body></w:document>`, text)
	if err := archive.Close(); err != nil {
		t.Fatalf("falha ao fechar DOCX: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao escrever DOCX: %v", err)
	}
}

func TestRoutineConsolidateProcessaEmParaleloEContinuaComFalhaParcial(t *testing.T) {
	inputDir := t.TempDir()
	writeRoutineDocx(t, filepath.Join(inputDir, "b-boa.docx"), "rotina boa b")
	writeRoutineDocx(t, filepath.Join(inputDir, "a-falha.docx"), "rotina falha")
	writeRoutineDocx(t, filepath.Join(inputDir, "c-boa.docx"), "rotina boa c")
	outputDir := t.TempDir()

	var inFlight int32
	var maxInFlight int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		current := atomic.AddInt32(&inFlight, 1)
		for {
			previous := atomic.LoadInt32(&maxInFlight)
			if current <= previous || atomic.CompareAndSwapInt32(&maxInFlight, previous, current) {
				break
			}
		}
		defer atomic.AddInt32(&inFlight, -1)
		if bytes.Contains(body, []byte("rotina falha")) {
			http.Error(w, `{"error":{"message":"falha simulada"}}`, http.StatusBadRequest)
			return
		}
		time.Sleep(250 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"[{\"data\":\"01/02/26\",\"campo\":\"O eu, o outro e o nós\",\"experiencia\":\"Atividade\"}]"}}]}`)
	}))
	defer server.Close()

	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = server.URL
	t.Cleanup(func() { ai.OpenRouterAPIURL = oldURL })
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "sk-test")

	oldOutputDir, oldModel, oldPrompt, oldWorkers := routineOutputDir, routineModelName, routinePromptDir, routineWorkers
	oldVerbose, oldQuiet, oldJSON := verboseFlag, quietFlag, jsonFlag
	routineOutputDir = outputDir
	routineModelName = ai.DefaultTextModel
	routinePromptDir = ""
	routineWorkers = 3
	verboseFlag, quietFlag, jsonFlag = false, false, false
	t.Cleanup(func() {
		routineOutputDir, routineModelName, routinePromptDir, routineWorkers = oldOutputDir, oldModel, oldPrompt, oldWorkers
		verboseFlag, quietFlag, jsonFlag = oldVerbose, oldQuiet, oldJSON
	})

	var stdout, stderr bytes.Buffer
	routineConsolidateCmd.SetOut(&stdout)
	routineConsolidateCmd.SetErr(&stderr)
	if err := routineConsolidateCmd.RunE(routineConsolidateCmd, []string{inputDir}); err != nil {
		t.Fatalf("consolidação não deveria abortar por falha parcial: %v", err)
	}
	if !strings.Contains(stdout.String(), "2 processada(s); 0 pulada(s); 1 falha(s)") {
		t.Fatalf("resumo deveria registrar falha parcial, obtido: %s", stdout.String())
	}
	outputs, err := filepath.Glob(filepath.Join(outputDir, "Campos_de_experiências_*.docx"))
	if err != nil || len(outputs) != 1 {
		t.Fatalf("esperava um relatório consolidado, obtido %v (erro: %v)", outputs, err)
	}
	if atomic.LoadInt32(&maxInFlight) < 2 {
		t.Fatalf("esperava chamadas concorrentes, máximo observado: %d", maxInFlight)
	}
}
