package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/vault"
)

func TestLibraryOutputIsIndexedImmediatelyAfterCommand(t *testing.T) {
	configureDOCXMergeTest(t)
	defer configureDOCXMergeTest(t)
	library := t.TempDir()
	t.Setenv("CARAMEL_LIBRARY_DIR", library)
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	store, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InitializeLibrary(context.Background(), library); err != nil {
		t.Fatal(err)
	}
	store.Close()
	dir := t.TempDir()
	first := filepath.Join(dir, "capa.docx")
	second := filepath.Join(dir, "atividade.docx")
	writeCLIForMergeDOCX(t, first, "capa")
	writeCLIForMergeDOCX(t, second, "atividade")
	args := []string{first, second}
	docxMergeCmd.SetContext(context.Background())
	startCLIExecution(docxMergeCmd, args)
	var stdout bytes.Buffer
	docxMergeCmd.SetOut(&stdout)
	if err := docxMergeCmd.RunE(docxMergeCmd, args); err != nil {
		t.Fatal(err)
	}
	store, err = vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	files, err := store.ListIndexedFiles(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range files {
		if strings.HasSuffix(file.Path, "capa_merged.docx") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("output não entrou no índice imediatamente: %#v", files)
	}
}
