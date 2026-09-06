package workflow

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"caramel/internal/workspace"
)

func TestImageServicePrintOperationsStayInsideProject(t *testing.T) {
	t.Setenv("CARAMEL_WORKSPACE_DIR", t.TempDir())
	project, err := workspace.CreateProject("Materiais")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "01-maca.png")
	writeWorkflowPNG(t, source)
	assets, err := project.ImportPaths([]string{source})
	if err != nil {
		t.Fatal(err)
	}

	service := ImageService{Project: project}
	if _, err := service.Cards(context.Background(), []string{assets[0].ID}); err != nil {
		t.Fatalf("Cards falhou: %v", err)
	}
	if _, err := service.TwoUp(context.Background(), []string{assets[0].ID}); err != nil {
		t.Fatalf("TwoUp falhou: %v", err)
	}

	reopened, err := workspace.OpenProject(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Runs) != 2 {
		t.Fatalf("esperava duas execuções persistidas, obteve %d", len(reopened.Runs))
	}
	for _, run := range reopened.Runs {
		if run.Status != "completed" || len(run.Artifacts) != 1 {
			t.Fatalf("execução inválida: %+v", run)
		}
		for _, artifact := range run.Artifacts {
			if !filepath.IsAbs(reopened.Resolve(artifact.Path)) {
				t.Fatalf("Resolve deveria produzir caminho absoluto: %s", artifact.Path)
			}
			if _, err := os.Stat(reopened.Resolve(artifact.Path)); err != nil {
				t.Fatalf("artefato não encontrado: %v", err)
			}
		}
	}
}

func writeWorkflowPNG(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 8), B: 80, A: 255})
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
