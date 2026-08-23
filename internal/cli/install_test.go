package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "origem.txt")
	dst := filepath.Join(dir, "destino.bin")

	content := []byte("conteúdo do binário do caramel ☕")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("falha ao criar arquivo de origem: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile falhou: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("arquivo de destino não foi criado: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("conteúdo diverge: esperado %d bytes, obtido %d bytes", len(content), len(got))
	}

	// Sobrescrita: copiar por cima de destino existente deve funcionar
	newContent := []byte("nova versão")
	if err := os.WriteFile(src, newContent, 0644); err != nil {
		t.Fatalf("falha ao reescrever origem: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile de sobrescrita falhou: %v", err)
	}
	got, _ = os.ReadFile(dst)
	if string(got) != string(newContent) {
		t.Errorf("sobrescrita deveria substituir o conteúdo antigo, obtido %q", got)
	}
}

func TestCopyFileOrigemInexistente(t *testing.T) {
	dir := t.TempDir()
	if err := copyFile(filepath.Join(dir, "nao-existe"), filepath.Join(dir, "dst")); err == nil {
		t.Error("copiar origem inexistente deveria retornar erro")
	}
}

func TestAddPathWindowsForaDoWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Windows o PowerShell existe; teste só cobre a falha fora dele")
	}
	if err := addPathWindows(t.TempDir()); err == nil {
		t.Error("sem powershell disponível a função deveria retornar erro")
	}
}
