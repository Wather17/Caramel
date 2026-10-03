package vault

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitializeLibraryAndSyncHeterogeneousFiles(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	library := filepath.Join(t.TempDir(), "Caramel")
	sources, err := v.InitializeLibrary(context.Background(), library)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].Role != SourceMaterials || sources[1].Role != SourceResults {
		t.Fatalf("fontes iniciais inesperadas: %#v", sources)
	}
	materials := filepath.Join(library, "materiais")
	writeIndexedFixture(t, filepath.Join(materials, "planejamento.docx"), "docx")
	writeIndexedFixture(t, filepath.Join(materials, "sub", "atividade.pdf"), "pdf")
	writeIndexedFixture(t, filepath.Join(materials, "sub", "imagem.PNG"), "png")
	writeIndexedFixture(t, filepath.Join(materials, "~$temporario.docx"), "lock")
	writeIndexedFixture(t, filepath.Join(materials, ".oculto"), "hidden")

	report, err := v.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Created != 3 || report.Files != 3 || len(report.Warnings) != 0 {
		t.Fatalf("relatório inicial inesperado: %+v", report)
	}
	files, err := v.ListIndexedFiles(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || files[0].ContentHash == "" {
		t.Fatalf("arquivos indexados inesperados: %#v", files)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name, "~$") || strings.HasPrefix(file.Name, ".") {
			t.Fatalf("arquivo temporário/oculto foi indexado: %s", file.Path)
		}
	}

	second, err := v.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Unchanged != 3 || second.Created != 0 || second.Updated != 0 {
		t.Fatalf("sync idempotente inesperado: %+v", second)
	}
}

func TestSyncTracksChangesMovesAndUnavailableFiles(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	root := t.TempDir()
	if _, err := v.AddSource(context.Background(), root, SourceExternal); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, "primeiro.docx")
	second := filepath.Join(root, "segundo.pdf")
	writeIndexedFixture(t, first, "primeira versão")
	writeIndexedFixture(t, second, "pdf")
	if _, err := v.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Millisecond)
	writeIndexedFixture(t, first, "segunda versão maior")
	moved := filepath.Join(root, "sub", "movido.pdf")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(second, moved); err != nil {
		t.Fatal(err)
	}
	report, err := v.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Updated != 1 || report.Moved != 1 || report.Unavailable != 0 {
		t.Fatalf("mudança/movimento inesperado: %+v", report)
	}

	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	report, err = v.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Unavailable != 1 {
		t.Fatalf("remoção não marcada: %+v", report)
	}
	all, err := v.ListIndexedFiles(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("histórico deveria preservar dois registros: %#v", all)
	}
}

func TestSyncPreservesIndexWhenSourceDisappears(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	root := filepath.Join(t.TempDir(), "externa")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIndexedFixture(t, filepath.Join(root, "arquivo.zip"), "zip")
	if _, err := v.AddSource(context.Background(), root, SourceExternal); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	report, err := v.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Warnings) == 0 {
		t.Fatal("fonte ausente deveria produzir aviso")
	}
	files, err := v.ListIndexedFiles(context.Background(), false)
	if err != nil || len(files) != 1 {
		t.Fatalf("último índice válido deveria ser preservado: %#v, %v", files, err)
	}
}

func TestSyncRecognizesMoveBetweenSources(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	base := t.TempDir()
	materials := filepath.Join(base, "materials")
	external := filepath.Join(base, "external")
	for _, dir := range []string{materials, external} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := v.AddSource(context.Background(), materials, SourceMaterials); err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddSource(context.Background(), external, SourceExternal); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(external, "atividade.docx")
	newPath := filepath.Join(materials, "atividade.docx")
	writeIndexedFixture(t, oldPath, "conteúdo")
	if _, err := v.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	report, err := v.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Moved != 1 || report.Created != 0 {
		t.Fatalf("movimento entre fontes não reconhecido: %+v", report)
	}
	files, err := v.ListIndexedFiles(context.Background(), true)
	if err != nil || len(files) != 1 || files[0].Path != newPath || !files[0].Available {
		t.Fatalf("localização movida inesperada: %#v, %v", files, err)
	}
}

func TestSyncIfStaleAndTouch(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	root := t.TempDir()
	path := filepath.Join(root, "atividade.docx")
	writeIndexedFixture(t, path, "docx")
	if _, err := v.AddSource(context.Background(), root, SourceExternal); err != nil {
		t.Fatal(err)
	}
	if _, synced, err := v.SyncIfStale(context.Background(), 30*time.Second); err != nil || !synced {
		t.Fatalf("primeiro stale sync: synced=%v err=%v", synced, err)
	}
	if _, synced, err := v.SyncIfStale(context.Background(), 30*time.Second); err != nil || synced {
		t.Fatalf("segundo stale sync: synced=%v err=%v", synced, err)
	}
	if err := v.TouchIndexedFile(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	files, err := v.ListIndexedFiles(context.Background(), false)
	if err != nil || len(files) != 1 || files[0].LastUsedAt == nil {
		t.Fatalf("uso não registrado: %#v, %v", files, err)
	}
}

func TestFullSyncRepairsHashWhenMetadataIsUnchanged(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	root := t.TempDir()
	path := filepath.Join(root, "mesmo-tamanho.bin")
	writeIndexedFixture(t, path, "AAAA")
	if _, err := v.AddSource(context.Background(), root, SourceExternal); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("BBBB"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	normal, err := v.Sync(context.Background(), false)
	if err != nil || normal.Updated != 0 {
		t.Fatalf("sync incremental deveria confiar nos metadados: %+v, %v", normal, err)
	}
	full, err := v.Sync(context.Background(), true)
	if err != nil || full.Updated != 1 {
		t.Fatalf("sync completo deveria reparar o hash: %+v, %v", full, err)
	}
}

func TestSearchIndexedFilesFiltersRanksAndExcludes(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	root := t.TempDir()
	older := filepath.Join(root, "Caderno Árvore.docx")
	newer := filepath.Join(root, "Caderno Água.docx")
	ignoredType := filepath.Join(root, "Caderno recente.pdf")
	for _, path := range []string{older, newer, ignoredType} {
		writeIndexedFixture(t, path, path)
	}
	base := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(older, base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, base.Add(time.Hour), base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddSource(context.Background(), root, SourceExternal); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	results, err := v.SearchIndexedFiles(context.Background(), IndexedFileQuery{Text: "caderno", Extensions: []string{"DOCX"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Path != newer || results[0].SourceRole != SourceExternal {
		t.Fatalf("ranking/filtro inesperado: %#v", results)
	}
	accentless, err := v.SearchIndexedFiles(context.Background(), IndexedFileQuery{Text: "arvore", Extensions: []string{".docx"}})
	if err != nil || len(accentless) != 1 || accentless[0].Path != older {
		t.Fatalf("busca sem acento inesperada: %#v, %v", accentless, err)
	}
	excluded, err := v.SearchIndexedFiles(context.Background(), IndexedFileQuery{Extensions: []string{"docx"}, Exclude: []string{newer}})
	if err != nil || len(excluded) != 1 || excluded[0].Path != older {
		t.Fatalf("exclusão inesperada: %#v, %v", excluded, err)
	}
}

func writeIndexedFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
