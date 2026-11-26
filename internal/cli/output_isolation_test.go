package cli

import (
	"os"
	"testing"
)

func isolateLocalOutputConfig(t *testing.T) {
	t.Helper()
	// A configuração global e o .env local podem definir uma biblioteca mesmo com a variável vazia.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CARAMEL_LIBRARY_DIR", "")
	t.Setenv("CARAMEL_VAULT_DIR", t.TempDir())
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Errorf("restaurar diretório: %v", err)
		}
	})
}
