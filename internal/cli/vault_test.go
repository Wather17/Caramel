package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVaultCommandsInitializeAddSyncAndReportStatus(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	t.Setenv("CARAMEL_LIBRARY_DIR", "")
	library := filepath.Join(t.TempDir(), "Caramel")
	external := filepath.Join(t.TempDir(), "docs-mae")
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "rotina.docx"), []byte("docx"), 0o600); err != nil {
		t.Fatal(err)
	}

	if output := runVaultCommandForTest(t, vaultInitCmd, []string{library}); !strings.Contains(output, "Biblioteca inicializada") {
		t.Fatalf("saída de init inesperada: %q", output)
	}
	for _, dir := range []string{"materiais", "resultados"} {
		if info, err := os.Stat(filepath.Join(library, dir)); err != nil || !info.IsDir() {
			t.Fatalf("pasta %s não criada: %v", dir, err)
		}
	}
	if output := runVaultCommandForTest(t, vaultSourceAddCmd, []string{external}); !strings.Contains(output, "Fonte externa adicionada") {
		t.Fatalf("saída de source add inesperada: %q", output)
	}
	if output := runVaultCommandForTest(t, vaultSourceListCmd, nil); !strings.Contains(output, external) || !strings.Contains(output, "external") {
		t.Fatalf("saída de source list inesperada: %q", output)
	}
	vaultSyncFull = false
	if output := runVaultCommandForTest(t, vaultSyncCmd, nil); !strings.Contains(output, "1 novo(s)") {
		t.Fatalf("saída de sync inesperada: %q", output)
	}
	if output := runVaultCommandForTest(t, vaultStatusCmd, nil); !strings.Contains(output, "3 fonte(s)") || !strings.Contains(output, "1 arquivo(s)") {
		t.Fatalf("saída de status inesperada: %q", output)
	}
}

func runVaultCommandForTest(t *testing.T, cmd *cobra.Command, args []string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, args); err != nil {
		t.Fatalf("%s falhou: %v (stderr=%s)", cmd.CommandPath(), err, stderr.String())
	}
	return stdout.String()
}
