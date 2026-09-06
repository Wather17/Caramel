package workspace

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateProjectAndImportImages(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CARAMEL_WORKSPACE_DIR", filepath.Join(root, "workspaces"))

	sourceDir := filepath.Join(root, "originais")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "01-maca.png")
	writeTestPNG(t, source, color.RGBA{R: 255, A: 255})

	project, err := CreateProject("Atividades de Frutas")
	if err != nil {
		t.Fatalf("CreateProject falhou: %v", err)
	}
	if project.ID != "atividades-de-frutas" {
		t.Fatalf("ID inesperado: %q", project.ID)
	}

	assets, err := project.ImportPaths([]string{sourceDir})
	if err != nil {
		t.Fatalf("ImportPaths falhou: %v", err)
	}
	if len(assets) != 1 || len(project.Assets) != 1 {
		t.Fatalf("esperava 1 ativo importado, obteve %d/%d", len(assets), len(project.Assets))
	}
	if _, err := os.Stat(project.Resolve(assets[0].Path)); err != nil {
		t.Fatalf("cópia do ativo não foi criada: %v", err)
	}

	duplicates, err := project.ImportPaths([]string{source})
	if err != nil {
		t.Fatalf("segunda importação falhou: %v", err)
	}
	if len(duplicates) != 0 || len(project.Assets) != 1 {
		t.Fatalf("importação duplicada deveria ser ignorada: %d ativos, total %d", len(duplicates), len(project.Assets))
	}

	reopened, err := OpenProject(project.ID)
	if err != nil {
		t.Fatalf("OpenProject falhou: %v", err)
	}
	if len(reopened.ImageFiles()) != 1 || reopened.Name != project.Name {
		t.Fatalf("projeto reaberto não preservou manifesto: %+v", reopened)
	}
}

func TestProjectRunHistoryAndOutputDirectories(t *testing.T) {
	t.Setenv("CARAMEL_WORKSPACE_DIR", t.TempDir())
	project, err := CreateProject("Histórico")
	if err != nil {
		t.Fatal(err)
	}
	run, err := project.BeginRun("cards", []string{"asset-1"}, map[string]string{"model": "modelo"})
	if err != nil {
		t.Fatal(err)
	}
	output := project.OutputDir("cards", run.ID)
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("diretório de saída não criado: %v", err)
	}
	artifact := Artifact{ID: "artifact-1", Name: "fichas.pdf", Path: "outputs/cards/" + run.ID + "/fichas.pdf", Kind: "pdf", Step: "cards"}
	if err := project.FinishRun(run.ID, "completed", []Artifact{artifact}, nil); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenProject(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Runs) != 1 || reopened.Runs[0].Status != "completed" || len(reopened.Runs[0].Artifacts) != 1 {
		t.Fatalf("histórico não persistido corretamente: %+v", reopened.Runs)
	}
}

func writeTestPNG(t *testing.T, path string, fill color.Color) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, fill)
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}
