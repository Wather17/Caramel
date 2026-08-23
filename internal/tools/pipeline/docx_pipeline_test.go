package pipeline

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
)

// cores de fixture: cinza tem saturação 0 (passa pela triagem local);
// vermelho vivo tem saturação alta (é pulado na triagem local)
var (
	corCinza        = color.RGBA{R: 128, G: 128, B: 128, A: 255}
	corVermelhoVivo = color.RGBA{R: 220, G: 30, B: 30, A: 255}
)

func pngBytes(t *testing.T, size int, c color.RGBA) []byte {
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

func buildTestDocx(t *testing.T, dir string, media []struct {
	name string
	data []byte
}) string {
	t.Helper()
	docxPath := filepath.Join(dir, "atividade.docx")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range []string{"[Content_Types].xml", "word/document.xml"} {
		w, err := zw.Create(entry)
		if err != nil {
			t.Fatalf("falha ao criar entrada %s: %v", entry, err)
		}
		w.Write([]byte("<?xml?><doc/>"))
	}
	for _, m := range media {
		w, err := zw.Create("word/media/" + m.name)
		if err != nil {
			t.Fatalf("falha ao criar mídia %s: %v", m.name, err)
		}
		w.Write(m.data)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("falha ao fechar zip: %v", err)
	}
	if err := os.WriteFile(docxPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao escrever .docx de teste: %v", err)
	}
	return docxPath
}

// mockColorizeAPI redireciona a OpenRouter para um servidor fake que responde
// sempre com um PNG válido (ou erro HTTP), contando as chamadas recebidas.
func mockColorizeAPI(t *testing.T, status int, hits *int) {
	t.Helper()
	payload := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,%s"}}]}}]}`,
		base64.StdEncoding.EncodeToString(pngBytes(t, 4, corVermelhoVivo)))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			*hits++
		}
		if status != http.StatusOK {
			http.Error(w, "erro simulado pelo teste", status)
			return
		}
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

func readZipEntry(t *testing.T, zipPath, name string) ([]byte, bool) {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("falha ao abrir zip %s: %v", zipPath, err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("falha ao abrir entrada %s: %v", name, err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("falha ao ler entrada %s: %v", name, err)
			}
			return data, true
		}
	}
	return nil, false
}

func TestRunDocxPipelineCaminhoFeliz(t *testing.T) {
	dir := t.TempDir()
	original := pngBytes(t, 8, corCinza)
	docxPath := buildTestDocx(t, dir, []struct {
		name string
		data []byte
	}{{name: "image1.png", data: original}})

	hits := 0
	mockColorizeAPI(t, http.StatusOK, &hits)

	res, err := RunDocxPipeline(docxPath, dir, "sk-test", "m/imagem", 0, false, "", true)
	if err != nil {
		t.Fatalf("RunDocxPipeline falhou: %v", err)
	}

	if res.TotalExtracted != 1 || res.TotalColorized != 1 || res.TotalSkipped != 0 || res.TotalFormatSkipped != 0 {
		t.Errorf("contadores inesperados: %+v", res)
	}
	if hits != 1 {
		t.Errorf("esperado 1 chamada à API, obtido %d", hits)
	}
	if res.RebuiltDocxPath == "" {
		t.Fatal("RebuiltDocxPath deveria estar preenchido")
	}
	if _, err := os.Stat(res.RebuiltDocxPath); err != nil {
		t.Fatalf(".docx reconstruído não existe: %v", err)
	}

	// A imagem substituída deve ser um PNG válido nas dimensões originais (8x8 após resize)
	data, ok := readZipEntry(t, res.RebuiltDocxPath, "word/media/image1.png")
	if !ok {
		t.Fatal("imagem substituída não encontrada dentro do .docx reconstruído")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("imagem reinserida deveria ser PNG válido: %v", err)
	}
	if got := img.Bounds().Dx(); got != 8 {
		t.Errorf("ResizeToMatch deveria manter 8px de largura, obtido %d", got)
	}
}

func TestRunDocxPipelinePulaFormatoNaoColorivel(t *testing.T) {
	dir := t.TempDir()
	docxPath := buildTestDocx(t, dir, []struct {
		name string
		data []byte
	}{
		{name: "image1.emf", data: []byte{0x01, 0x00, 0x00, 0x00}},
		{name: "image2.png", data: pngBytes(t, 6, corCinza)},
	})

	hits := 0
	mockColorizeAPI(t, http.StatusOK, &hits)

	res, err := RunDocxPipeline(docxPath, dir, "sk-test", "m/imagem", 0, false, "", true)
	if err != nil {
		t.Fatalf("RunDocxPipeline falhou: %v", err)
	}

	if res.TotalFormatSkipped != 1 || len(res.FormatSkipped) != 1 || res.FormatSkipped[0].OriginalName != "image1.emf" {
		t.Errorf("esperado image1.emf pulada por formato, obtido %+v", res.FormatSkipped)
	}
	if res.TotalColorized != 1 || hits != 1 {
		t.Errorf("apenas o PNG deveria ir para a API (coloridas=%d, chamadas=%d)", res.TotalColorized, hits)
	}
}

func TestRunDocxPipelineMinSizeRetornaCedo(t *testing.T) {
	dir := t.TempDir()
	docxPath := buildTestDocx(t, dir, []struct {
		name string
		data []byte
	}{{name: "pequena.png", data: pngBytes(t, 4, corCinza)}})

	hits := 0
	mockColorizeAPI(t, http.StatusOK, &hits)

	res, err := RunDocxPipeline(docxPath, dir, "sk-test", "m/imagem", 10_000_000, false, "", true)
	if err != nil {
		t.Fatalf("RunDocxPipeline falhou: %v", err)
	}

	if res.TotalExtracted != 0 || res.TotalSkipped != 1 || len(res.SkippedImages) != 1 {
		t.Errorf("esperado extração vazia com 1 imagem pulada por tamanho, obtido %+v", res)
	}
	if hits != 0 {
		t.Errorf("nenhuma chamada deveria ocorrer sem imagens extraídas, obtido %d", hits)
	}
	if _, err := os.Stat(res.RebuiltDocxPath); !os.IsNotExist(err) {
		t.Error(".docx reconstruído não deveria existir sem substituições")
	}
}

func TestRunDocxPipelineTriagemLocalPulaJaColorida(t *testing.T) {
	dir := t.TempDir()
	docxPath := buildTestDocx(t, dir, []struct {
		name string
		data []byte
	}{{name: "colorida.png", data: pngBytes(t, 16, corVermelhoVivo)}})

	hits := 0
	mockColorizeAPI(t, http.StatusOK, &hits)

	// Triagem ativada: a análise local de saturação deve pular sem chamar a API
	res, err := RunDocxPipeline(docxPath, dir, "sk-test", "m/imagem", 0, false, "m/triagem", false)
	if err != nil {
		t.Fatalf("RunDocxPipeline falhou: %v", err)
	}

	if hits != 0 {
		t.Errorf("triagem local deve decidir sem custo de API, obtido %d chamadas", hits)
	}
	if res.TotalTriageSkipped != 1 || len(res.TriageSkipped) != 1 {
		t.Fatalf("esperado 1 imagem pulada pela triagem, obtido %+v", res)
	}
	skip := res.TriageSkipped[0]
	if skip.Stage != "local" || skip.Name != "colorida.png" {
		t.Errorf("skip inesperado: %+v", skip)
	}
	if res.TotalColorized != 0 {
		t.Errorf("nenhuma imagem deveria ser colorida, obtido %d", res.TotalColorized)
	}
}

func TestRunDocxPipelineAPIFalhaNaoAborta(t *testing.T) {
	dir := t.TempDir()
	docxPath := buildTestDocx(t, dir, []struct {
		name string
		data []byte
	}{{name: "image1.png", data: pngBytes(t, 8, corCinza)}})

	// 400 não é retransmitido (erro permanente): falha rápida sem sleeps de retry
	mockColorizeAPI(t, http.StatusBadRequest, nil)

	res, err := RunDocxPipeline(docxPath, dir, "sk-test", "m/imagem", 0, false, "", true)
	if err != nil {
		t.Fatalf("falha da API não deveria abortar o pipeline, obtido erro: %v", err)
	}
	if res.TotalColorized != 0 {
		t.Errorf("nada deveria ser colorido com API em erro, obtido %d", res.TotalColorized)
	}
}

func TestRunDocxPipelineSelectedSoSelecionadas(t *testing.T) {
	dir := t.TempDir()
	imgA := pngBytes(t, 8, corCinza)
	imgB := pngBytes(t, 6, corCinza)
	docxPath := buildTestDocx(t, dir, []struct {
		name string
		data []byte
	}{
		{name: "a.png", data: imgA},
		{name: "b.png", data: imgB},
	})

	hits := 0
	mockColorizeAPI(t, http.StatusOK, &hits)

	selecionadas := []docx.ExtractedImage{
		{OriginalName: "a.png", PathInZip: "word/media/a.png", Size: int64(len(imgA)), Format: "png"},
	}

	res, err := RunDocxPipelineSelected(docxPath, dir, "sk-test", "m/imagem", selecionadas, false, "", true)
	if err != nil {
		t.Fatalf("RunDocxPipelineSelected falhou: %v", err)
	}

	if res.TotalExtracted != 1 || res.TotalColorized != 1 || hits != 1 {
		t.Fatalf("somente 'a.png' deveria ser processada (extraídas=%d, coloridas=%d, chamadas=%d)", res.TotalExtracted, res.TotalColorized, hits)
	}

	// a.png substituída por PNG válido; b.png preservada byte a byte
	rebuilt := res.RebuiltDocxPath
	if data, ok := readZipEntry(t, rebuilt, "word/media/a.png"); !ok {
		t.Error("a.png deveria existir no docx reconstruído")
	} else if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("a.png substituída deveria ser PNG válido: %v", err)
	}
	if data, ok := readZipEntry(t, rebuilt, "word/media/b.png"); !ok {
		t.Error("b.png deveria ser preservada no docx reconstruído")
	} else if !bytes.Equal(data, imgB) {
		t.Error("b.png não selecionada deveria permanecer intacta")
	}
}

func TestRunDocxPipelineSelectedSemSelecao(t *testing.T) {
	dir := t.TempDir()

	res, err := RunDocxPipelineSelected(filepath.Join(dir, "atividade.docx"), dir, "sk-test", "m/imagem", nil, false, "", true)
	if err != nil {
		t.Fatalf("RunDocxPipelineSelected falhou: %v", err)
	}
	if res.TotalColorized != 0 || res.DocxPath == "" {
		t.Errorf("seleção vazia deveria retornar resultado vazio imediato, obtido %+v", res)
	}
}

func TestRunDocxPipelineSelectedPulaFormatoNaoColorivel(t *testing.T) {
	dir := t.TempDir()
	imgA := pngBytes(t, 8, corCinza)
	docxPath := buildTestDocx(t, dir, []struct {
		name string
		data []byte
	}{
		{name: "a.emf", data: []byte{0x01, 0x00}},
		{name: "b.png", data: imgA},
	})

	hits := 0
	mockColorizeAPI(t, http.StatusOK, &hits)

	selecionadas := []docx.ExtractedImage{
		{OriginalName: "a.emf", PathInZip: "word/media/a.emf", Size: 2, Format: "emf"},
		{OriginalName: "b.png", PathInZip: "word/media/b.png", Size: int64(len(imgA)), Format: "png"},
	}

	res, err := RunDocxPipelineSelected(docxPath, dir, "sk-test", "m/imagem", selecionadas, false, "", true)
	if err != nil {
		t.Fatalf("RunDocxPipelineSelected falhou: %v", err)
	}
	if res.TotalFormatSkipped != 1 || res.FormatSkipped[0].OriginalName != "a.emf" {
		t.Errorf("esperado a.emf pulada por formato, obtido %+v", res.FormatSkipped)
	}
	if hits != 1 || res.TotalColorized != 1 {
		t.Errorf("apenas o PNG deveria ser colorido (coloridas=%d, chamadas=%d)", res.TotalColorized, hits)
	}
}
