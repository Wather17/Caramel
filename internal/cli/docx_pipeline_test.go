package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
	"caramel/internal/tools/pipeline"
)

func TestPrintTriageSummaryNil(t *testing.T) {
	out := captureStdout(t, func() { printTriageSummary(nil) })
	if out != "" {
		t.Errorf("resumo nulo não deveria imprimir nada, obtido: %q", out)
	}
}

func TestPrintTriageSummarySemPulos(t *testing.T) {
	out := captureStdout(t, func() {
		printTriageSummary(&pipeline.PipelineResult{TotalColorized: 3})
	})
	if out != "" {
		t.Errorf("sem pulos não deveria imprimir nada, obtido: %q", out)
	}
}

func TestPrintTriageSummaryFormatoNaoColorivel(t *testing.T) {
	res := &pipeline.PipelineResult{
		TotalFormatSkipped: 2,
		FormatSkipped: []docx.ExtractedImage{
			{OriginalName: "image1.emf", Format: "emf"},
			{OriginalName: "image2.wmf", Format: "wmf"},
		},
	}

	out := captureStdout(t, func() { printTriageSummary(res) })

	if !strings.Contains(out, "(formato não colorível): 2") {
		t.Errorf("deveria anunciar o total de formatos pulados, obtido: %q", out)
	}
	if !strings.Contains(out, "image1.emf (emf)") || !strings.Contains(out, "image2.wmf (wmf)") {
		t.Errorf("deveria listar nome e formato de cada imagem pulada, obtido: %q", out)
	}
}

func TestPrintTriageSummaryTriagemLLMELocal(t *testing.T) {
	res := &pipeline.PipelineResult{
		TotalTriageSkipped: 2,
		TriageSkipped: []ai.TriageSkipInfo{
			{Name: "image3.png", Stage: "local", Reason: "já parece colorida"},
			{Name: "image4.png", Stage: "llm", Reason: "foto do mundo real"},
		},
	}

	out := captureStdout(t, func() { printTriageSummary(res) })

	if !strings.Contains(out, "economia de API): 2") {
		t.Errorf("deveria anunciar o total pulado pela triagem, obtido: %q", out)
	}
	if !strings.Contains(out, "[análise local]: já parece colorida") {
		t.Errorf("stage 'local' deveria ser exibido como 'análise local', obtido: %q", out)
	}
	if !strings.Contains(out, "[LLM]: foto do mundo real") {
		t.Errorf("stage llm deveria ser exibido como LLM, obtido: %q", out)
	}
}

func TestRunProcessDocxExtensaoInvalida(t *testing.T) {
	err := RunProcessDocx(ProcessDocxOptions{DocxPath: "atividade.txt"})
	if err == nil || !strings.Contains(err.Error(), ".docx") {
		t.Errorf("extensão inválida deveria ser rejeitada, obtido: %v", err)
	}
}

func TestRunProcessDocxSemChaveAPI(t *testing.T) {
	dir := t.TempDir()
	docxPath := buildCliTestDocx(t, dir, nil)
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", dir) // isola do .env global da máquina

	err := RunProcessDocx(ProcessDocxOptions{DocxPath: docxPath})
	if err == nil || !strings.Contains(err.Error(), "chave de API") {
		t.Errorf("ausência de chave de API deveria bloquear o pipeline, obtido: %v", err)
	}
}

func TestRunProcessDocxInterativoSemImagens(t *testing.T) {
	dir := t.TempDir()
	docxPath := buildCliTestDocx(t, dir, nil)
	t.Setenv("OPENROUTER_API_KEY", "sk-test")

	out := captureStdout(t, func() {
		if err := RunProcessDocx(ProcessDocxOptions{DocxPath: docxPath, Interactive: true}); err != nil {
			t.Errorf("docx sem imagens não deveria falhar: %v", err)
		}
	})
	if !strings.Contains(out, "Nenhuma imagem foi encontrada") {
		t.Errorf("deveria avisar sobre a ausência de imagens, obtido: %q", out)
	}
}

func TestRunProcessDocxAutomatizadoFeliz(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "saida")
	docxPath := buildCliTestDocx(t, dir, []struct {
		name string
		data []byte
	}{{name: "image1.png", data: cliPngBytes(t, 8, corCinzaCli)}})
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	mockOpenRouterCLI(t)

	err := RunProcessDocx(ProcessDocxOptions{DocxPath: docxPath, OutputDir: outDir, MinSize: "0", NoTriage: true})
	if err != nil {
		t.Fatalf("RunProcessDocx falhou: %v", err)
	}

	rebuilt := filepath.Join(outDir, "atividade colorida.docx")
	if _, err := os.Stat(rebuilt); err != nil {
		t.Errorf(".docx colorido deveria existir em %s: %v", rebuilt, err)
	}
}

var corCinzaCli = color.RGBA{R: 128, G: 128, B: 128, A: 255}

func cliPngBytes(t *testing.T, size int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("falha ao gerar PNG de teste: %v", err)
	}
	return buf.Bytes()
}

// buildCliTestDocx monta um .docx mínimo (zip com word/media) para os testes do CLI
func buildCliTestDocx(t *testing.T, dir string, media []struct {
	name string
	data []byte
}) string {
	t.Helper()
	docxPath := filepath.Join(dir, "atividade.docx")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("[Content_Types].xml")
	w.Write([]byte("<?xml?><doc/>"))
	for _, m := range media {
		wm, _ := zw.Create("word/media/" + m.name)
		wm.Write(m.data)
	}
	zw.Close()

	if err := os.WriteFile(docxPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao escrever .docx de teste: %v", err)
	}
	return docxPath
}

// mockOpenRouterCLI redireciona a API para um servidor fake que responde com um PNG válido
func mockOpenRouterCLI(t *testing.T) {
	t.Helper()
	payload := fmt.Sprintf(`{"choices":[{"message":{"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,%s"}}]}}]}`,
		base64.StdEncoding.EncodeToString(cliPngBytes(t, 4, color.RGBA{R: 220, G: 30, B: 30, A: 255})))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, payload)
	}))
	oldURL := ai.OpenRouterAPIURL
	ai.OpenRouterAPIURL = srv.URL
	t.Cleanup(func() {
		ai.OpenRouterAPIURL = oldURL
		srv.Close()
	})
}
