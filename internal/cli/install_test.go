package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"caramel/internal/shell"
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

func TestConfigureLinuxPathCreatesAndPreservesProfiles(t *testing.T) {
	for _, test := range []struct {
		name          string
		shell         string
		zdotdir       string
		xdgConfigHome string
		wantProfile   string
		wantSyntax    string
	}{
		{name: "bash", shell: "/bin/bash", wantProfile: ".bashrc", wantSyntax: "export PATH='"},
		{name: "zsh", shell: "/usr/bin/zsh", zdotdir: "profiles", wantProfile: filepath.Join("profiles", ".zshrc"), wantSyntax: "export PATH='"},
		{name: "fish", shell: "/usr/bin/fish", wantProfile: filepath.Join(".config", "fish", "config.fish"), wantSyntax: "set -gx PATH '"},
		{name: "fish xdg", shell: "/usr/bin/fish", xdgConfigHome: "config", wantProfile: filepath.Join("config", "fish", "config.fish"), wantSyntax: "set -gx PATH '"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			installDir := filepath.Join(home, ".local", "bin")
			zdotdir := ""
			if test.zdotdir != "" {
				zdotdir = filepath.Join(home, test.zdotdir)
			}
			configHome := ""
			if test.xdgConfigHome != "" {
				configHome = filepath.Join(home, test.xdgConfigHome)
			}
			pathEnv := filepath.Join(home, "bin") + string(os.PathListSeparator) + "/usr/bin"
			update, err := configureLinuxPath(home, installDir, pathEnv, test.shell, zdotdir, configHome)
			if err != nil {
				t.Fatalf("configurar PATH falhou: %v", err)
			}
			if !update.Updated {
				t.Fatal("primeira configuração deveria atualizar o perfil")
			}
			wantProfile := filepath.Join(home, test.wantProfile)
			if update.Profile.Path != wantProfile {
				t.Fatalf("perfil inesperado: %q", update.Profile.Path)
			}
			contents, err := os.ReadFile(wantProfile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(contents), test.wantSyntax) || !strings.Contains(string(contents), "# >>> caramel path >>>") {
				t.Fatalf("perfil não contém o bloco esperado: %s", contents)
			}

			if err := os.WriteFile(wantProfile, append([]byte("# conteúdo externo\n"), contents...), 0o600); err != nil {
				t.Fatal(err)
			}
			second, err := configureLinuxPath(home, installDir, pathEnv, test.shell, zdotdir, configHome)
			if err != nil {
				t.Fatalf("repetir configuração falhou: %v", err)
			}
			if !second.AlreadyConfigured {
				t.Fatal("repetir configuração deveria ser idempotente")
			}
			final, err := os.ReadFile(wantProfile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(final), "# conteúdo externo\n") {
				t.Fatalf("conteúdo externo foi removido: %s", final)
			}
			if strings.Count(string(final), "# >>> caramel path >>>") != 1 {
				t.Fatalf("bloco de PATH duplicado: %s", final)
			}
		})
	}
}

func TestConfigureLinuxPathRecognizesEquivalentExistingEntries(t *testing.T) {
	for _, test := range []struct {
		name          string
		shell         string
		content       string
		xdgConfigHome string
	}{
		{name: "bash home", shell: "bash", content: "export PATH=\"$HOME/.local/bin:$PATH\"\n"},
		{name: "zsh absolute", shell: "zsh", content: "PATH='/tmp/home/.local/bin':$PATH\n"},
		{name: "fish home", shell: "fish", content: "set -gx PATH $HOME/.local/bin $PATH\n"},
		{name: "fish xdg", shell: "fish", content: "set -gx PATH $HOME/.local/bin $PATH\n", xdgConfigHome: "config"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			installDir := filepath.Join(home, ".local", "bin")
			configHome := ""
			if test.xdgConfigHome != "" {
				configHome = filepath.Join(home, test.xdgConfigHome)
			}
			profileManager := shell.NewProfileManager(home, test.shell, "", configHome, installDir)
			profile, err := profileManager.Profile()
			if err != nil {
				t.Fatal(err)
			}
			content := test.content
			if strings.Contains(content, "/tmp/home") {
				content = strings.ReplaceAll(content, "/tmp/home", home)
			}
			if err := os.MkdirAll(filepath.Dir(profile.Path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(profile.Path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(profile.Path)
			update, err := configureLinuxPath(home, installDir, "/usr/bin", test.shell, "", configHome)
			if err != nil {
				t.Fatal(err)
			}
			if !update.AlreadyConfigured {
				t.Fatal("entrada equivalente deveria ser reconhecida")
			}
			after, _ := os.ReadFile(profile.Path)
			if string(before) != string(after) {
				t.Fatalf("perfil foi alterado apesar da entrada existente: antes=%q depois=%q", before, after)
			}
		})
	}
}

func TestConfigureLinuxPathRejectsUnknownShellWithoutWriting(t *testing.T) {
	home := t.TempDir()
	_, err := configureLinuxPath(home, filepath.Join(home, ".local", "bin"), "/usr/bin", "tcsh", "", "")
	if err == nil || !strings.Contains(err.Error(), "bash, zsh ou fish") {
		t.Fatalf("erro de shell inesperado: %v", err)
	}
	entries, readErr := os.ReadDir(home)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("shell desconhecido não deveria criar arquivos: %#v", entries)
	}
}

func TestConfigureLinuxPathSkipsProfileWhenProcessPathAlreadyContainsInstallDir(t *testing.T) {
	home := t.TempDir()
	installDir := filepath.Join(home, ".local", "bin")
	update, err := configureLinuxPath(
		home,
		installDir,
		installDir+string(os.PathListSeparator)+"/usr/bin",
		"bash",
		"",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !update.AlreadyConfigured {
		t.Fatal("PATH do processo já configurado deveria ser reconhecido")
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); !os.IsNotExist(err) {
		t.Fatalf("perfil não deveria ser criado quando PATH já contém o diretório: %v", err)
	}
}

func TestConfigureLinuxPathReportsProfileWriteFailure(t *testing.T) {
	home := t.TempDir()
	profilePath := filepath.Join(home, ".bashrc")
	if err := os.Mkdir(profilePath, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := configureLinuxPath(home, filepath.Join(home, ".local", "bin"), "/usr/bin", "bash", "", "")
	if err == nil {
		t.Fatal("perfil inválido deveria retornar erro acionável")
	}
}
