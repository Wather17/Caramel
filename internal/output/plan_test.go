package output

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanPublishesSingleByLocalDayAndCategory(t *testing.T) {
	root := t.TempDir()
	started := time.Date(2026, 10, 3, 23, 59, 0, 0, time.FixedZone("local", -5*60*60))
	plan, err := NewPlan(PlanRequest{LibraryRoot: root, Category: "pdf", Shape: ShapeSingle, BaseName: "apostila", Extension: ".pdf", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Cleanup()
	if err := os.WriteFile(plan.WorkPath(), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := plan.Publish([]string{plan.WorkPath()})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "resultados", "2026-10-03", "pdf", "apostila.pdf")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("outputs=%#v, esperava %s", paths, want)
	}
}

func TestPlanPublishesBundleAndUsesDeterministicCollisionSuffix(t *testing.T) {
	root := t.TempDir()
	started := time.Date(2026, 10, 3, 10, 0, 0, 0, time.Local)
	first, err := NewPlan(PlanRequest{LibraryRoot: root, Category: "docx", Shape: ShapeBundle, BaseName: "atividade_split", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Cleanup()
	second, err := NewPlan(PlanRequest{LibraryRoot: root, Category: "docx", Shape: ShapeBundle, BaseName: "atividade_split", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Cleanup()
	if err := os.MkdirAll(first.WorkPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(first.WorkPath(), "parte.docx")
	if err := os.WriteFile(part, []byte("docx"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Publish([]string{part}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(second.WorkPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	secondPart := filepath.Join(second.WorkPath(), "parte.docx")
	if err := os.WriteFile(secondPart, []byte("outro"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Publish([]string{secondPart}); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "resultados", "2026-10-03", "docx", "atividade_split-2")
	if second.FinalPath() != want {
		t.Fatalf("destino=%s, esperava %s", second.FinalPath(), want)
	}
}

func TestPlanDoesNotCreateCategoryWithoutArtifacts(t *testing.T) {
	root := t.TempDir()
	plan, err := NewPlan(PlanRequest{LibraryRoot: root, Category: "imagens", Shape: ShapeBundle, BaseName: "vazio", StartedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Cleanup()
	if err := os.MkdirAll(plan.WorkPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Publish(nil); err == nil {
		t.Fatal("pacote vazio deveria falhar")
	}
	if _, err := os.Stat(filepath.Join(root, "resultados")); !os.IsNotExist(err) {
		t.Fatalf("árvore pública não deveria existir, erro=%v", err)
	}
}
