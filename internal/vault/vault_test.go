package vault

import (
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/workspace"
)

func TestInferFilenameMetadata(t *testing.T) {
	tests := []struct {
		name     string
		category string
		tags     []string
	}{
		{name: "atividade de Ciências 01.png", category: "atividade", tags: []string{"ciencias"}},
		{name: "at-ciencias-recorte.png", category: "atividade", tags: []string{"ciencias", "recorte"}},
		{name: "sequência didática animais.pdf", category: "sequencia-didatica", tags: []string{"animais"}},
		{name: "folha_frutas_02.png", category: "folha", tags: []string{"frutas"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := inferFilenameMetadata(test.name)
			if metadata.Category != test.category {
				t.Fatalf("categoria inesperada: %q", metadata.Category)
			}
			if strings.Join(metadata.Tags, ",") != strings.Join(test.tags, ",") {
				t.Fatalf("tags inesperadas: %v", metadata.Tags)
			}
		})
	}
}

func TestImportInfersAndMergesFilenameMetadata(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", t.TempDir())
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "at ciencias 01.png")
	secondPath := filepath.Join(dir, "sequencia didatica animais.png")
	writeVaultPNG(t, firstPath)
	content, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	first, err := v.ImportFile(context.Background(), firstPath, "", []string{"importado"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Material.Category != "atividade" || !contains(first.Material.Tags, "ciencias") || !contains(first.Material.Tags, "importado") {
		t.Fatalf("metadados inferidos inesperados: %+v", first.Material)
	}
	second, err := v.ImportFile(context.Background(), secondPath, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.Material.ID != first.Material.ID || second.Material.Category != "atividade" || !contains(second.Material.Tags, "animais") {
		t.Fatalf("deduplicação não preservou/mesclou metadados: %+v", second.Material)
	}

	results, err := v.Search(context.Background(), SearchOptions{Query: "ciencias", Category: "atividade"})
	if err != nil || len(results) != 1 {
		t.Fatalf("busca por metadados falhou: %v, %d", err, len(results))
	}
}

func TestOpenMigratesCategoryColumn(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CARAMEL_VAULT_DIR", root)
	if err := os.MkdirAll(filepath.Join(root, "objects"), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "vault.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE schema_versions (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE materials (
 id TEXT PRIMARY KEY, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
 kind TEXT NOT NULL, extension TEXT NOT NULL, content_hash TEXT NOT NULL UNIQUE,
 version INTEGER NOT NULL DEFAULT 1, size INTEGER NOT NULL, object_path TEXT NOT NULL UNIQUE,
 source_name TEXT NOT NULL DEFAULT '', source_path TEXT NOT NULL DEFAULT '', archived_at TEXT,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE material_tags (material_id TEXT NOT NULL, tag TEXT NOT NULL, PRIMARY KEY(material_id, tag));
INSERT INTO schema_versions(version, applied_at) VALUES (1, '2026-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	var category string
	if err := v.db.QueryRow("SELECT category FROM materials LIMIT 1").Scan(&category); err != sql.ErrNoRows {
		t.Fatalf("schema legado não foi migrado como esperado: %v", err)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func TestVaultImportSearchCollectionAndArchive(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", t.TempDir())
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	source := filepath.Join(t.TempDir(), "maca.png")
	writeVaultPNG(t, source)
	first, err := v.ImportFile(context.Background(), source, "Maçã", []string{"frutas", "alfabetização"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := v.ImportFile(context.Background(), source, "Outro título", []string{"outro"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || first.Material.ID != second.Material.ID {
		t.Fatalf("deduplicação inválida: first=%+v second=%+v", first, second)
	}

	results, err := v.Search(context.Background(), SearchOptions{Query: "maçã", Tag: "frutas"})
	if err != nil || len(results) != 1 {
		t.Fatalf("busca não encontrou material: %v, %d resultados", err, len(results))
	}
	collection, err := v.CreateCollection(context.Background(), "Sessão de frutas", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.AddToCollection(context.Background(), collection.ID, first.Material.ID); err != nil {
		t.Fatal(err)
	}
	collectionMaterials, err := v.CollectionMaterials(context.Background(), collection.ID)
	if err != nil || len(collectionMaterials) != 1 {
		t.Fatalf("coleção inválida: %v, %d materiais", err, len(collectionMaterials))
	}

	run, err := v.StartRun(context.Background(), collection.ID, "cards", nil, []string{first.Material.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.AddDerivation(context.Background(), run.ID, []string{first.Material.ID}, []string{first.Material.ID}); err != nil {
		t.Fatal(err)
	}
	if err := v.FinishRun(context.Background(), run.ID, "completed", []string{first.Material.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if err := v.ArchiveMaterial(context.Background(), first.Material.ID); err != nil {
		t.Fatal(err)
	}
	active, err := v.Search(context.Background(), SearchOptions{Query: "maçã"})
	if err != nil || len(active) != 0 {
		t.Fatalf("material arquivado apareceu na busca ativa: %v, %d", err, len(active))
	}
	archived, err := v.Search(context.Background(), SearchOptions{Query: "maçã", IncludeArchived: true})
	if err != nil || len(archived) != 1 || archived[0].ArchivedAt == nil {
		t.Fatalf("material arquivado não foi preservado: %v, %+v", err, archived)
	}
}

func TestMigrateLegacyProjectsIsIdempotent(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CARAMEL_WORKSPACE_DIR", filepath.Join(root, "legacy"))
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(root, "vault"))

	project, err := workspace.CreateProject("Projeto antigo")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "01-gato.png")
	writeVaultPNG(t, source)
	assets, err := project.ImportPaths([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	run, err := project.BeginRun("cards", []string{assets[0].ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(project.Directory, "outputs", "cards.pdf")
	writeVaultPNGColor(t, artifactPath, color.RGBA{R: 220, G: 40, B: 80, A: 255})
	run.Artifacts = []workspace.Artifact{{ID: "artifact-old", Name: "cards.pdf", Path: "outputs/cards.pdf", Kind: "pdf", Step: "cards"}}
	if err := project.FinishRun(run.ID, "completed", run.Artifacts, nil); err != nil {
		t.Fatal(err)
	}

	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	first, err := v.MigrateLegacyProjects(context.Background())
	if err != nil || first.Projects != 1 || first.Runs != 1 {
		t.Fatalf("migração inválida: %+v, %v", first, err)
	}
	second, err := v.MigrateLegacyProjects(context.Background())
	if err != nil || second.Projects != 0 || second.Runs != 0 {
		t.Fatalf("migração não idempotente: %+v, %v", second, err)
	}
	materials, err := v.Search(context.Background(), SearchOptions{IncludeArchived: true})
	if err != nil || len(materials) != 2 {
		t.Fatalf("materiais migrados inesperados: %v, %d", err, len(materials))
	}
}

func writeVaultPNG(t *testing.T, path string) {
	writeVaultPNGColor(t, path, color.RGBA{R: 0, G: 0, B: 100, A: 255})
}

func writeVaultPNGColor(t *testing.T, path string, base color.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: base.R + uint8(x*2), G: base.G + uint8(y*2), B: base.B, A: base.A})
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
