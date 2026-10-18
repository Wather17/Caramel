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

func TestCopyFileQuandoOrigemEDestinoSaoOMesmo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "caramel")
	content := []byte("binário instalado")
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	if err := copyFile(path, path); err != nil {
		t.Fatalf("copiar o arquivo para ele mesmo deveria ser seguro: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("falha ao ler arquivo: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("o arquivo foi alterado ao copiar para ele mesmo: %q", got)
	}
}

func TestInstallMelAliasLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("o alias do Windows usa um launcher .cmd")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "caramel"), []byte("binário"), 0755); err != nil {
		t.Fatalf("falha ao criar binário: %v", err)
	}

	if warning, err := installMelAlias(dir, "caramel"); err != nil {
		t.Fatalf("criar alias falhou: %v", err)
	} else if warning != "" {
		t.Fatalf("criar alias não deveria gerar aviso: %s", warning)
	}

	target, err := os.Readlink(filepath.Join(dir, "mel"))
	if err != nil {
		t.Fatalf("alias não foi criado como link simbólico: %v", err)
	}
	if target != "caramel" {
		t.Fatalf("alvo inesperado do alias: %q", target)
	}

	if warning, err := installMelAlias(dir, "caramel"); err != nil || warning != "" {
		t.Fatalf("repetir a instalação do alias deveria ser idempotente: warning=%q err=%v", warning, err)
	}
}

func TestInstallMelAliasPreservaArquivoExistente(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("o alias do Windows usa um launcher .cmd")
	}

	dir := t.TempDir()
	aliasPath := filepath.Join(dir, "mel")
	if err := os.WriteFile(aliasPath, []byte("alias do usuário"), 0644); err != nil {
		t.Fatalf("falha ao criar alias existente: %v", err)
	}

	warning, err := installMelAlias(dir, "caramel")
	if err != nil {
		t.Fatalf("inspecionar alias existente falhou: %v", err)
	}
	if warning == "" {
		t.Fatal("alias não gerenciado deveria gerar aviso")
	}
	contents, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatalf("falha ao ler alias preservado: %v", err)
	}
	if string(contents) != "alias do usuário" {
		t.Fatalf("alias existente foi alterado: %q", contents)
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
