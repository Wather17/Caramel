package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

func TestVaultCompletionFiltersRanksAndAvoidsRepeatedInputs(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	root := filepath.Join(t.TempDir(), "Materiais da Mãe")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	docxPath := filepath.Join(root, "Árvore nova.docx")
	pdfPath := filepath.Join(root, "Árvore nova.pdf")
	for path, contents := range map[string]string{docxPath: "docx", pdfPath: "pdf"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddSource(context.Background(), root, vault.SourceExternal); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	store.Close()

	cmd := &cobra.Command{}
	complete := completeVaultFiles(".docx")
	items, directive := complete(cmd, nil, "arvore")
	if len(items) != 1 || !strings.HasPrefix(items[0], docxPath+"\tfonte externa") {
		t.Fatalf("sugestões inesperadas: %#v", items)
	}
	wantDirective := cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder
	if directive != wantDirective {
		t.Fatalf("diretiva=%v, esperava %v", directive, wantDirective)
	}
	items, directive = complete(cmd, []string{docxPath}, "")
	if len(items) != 0 || directive != cobra.ShellCompDirectiveDefault {
		t.Fatalf("entrada repetida deveria cair no filesystem: %#v, %v", items, directive)
	}
}

func TestTouchIndexedInputsRecordsSuccessfulUse(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	root := t.TempDir()
	path := filepath.Join(root, "atividade.docx")
	if err := os.WriteFile(path, []byte("docx"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddSource(context.Background(), root, vault.SourceExternal); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	store.Close()

	cmd := &cobra.Command{Annotations: map[string]string{vaultInputAnnotation: "args"}}
	touchIndexedInputs(cmd, []string{path})
	store, err = vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	files, err := store.ListIndexedFiles(context.Background(), false)
	if err != nil || len(files) != 1 || files[0].LastUsedAt == nil {
		t.Fatalf("uso não registrado: %#v, %v", files, err)
	}
}
