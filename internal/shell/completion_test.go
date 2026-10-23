package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionManagerEnableStatusDisablePreservesProfiles(t *testing.T) {
	for _, test := range []struct {
		name          string
		shell         string
		zdotdir       bool
		xdgConfigHome bool
		wantCommand   string
	}{
		{name: "bash", shell: "bash", wantCommand: "caramel completion bash"},
		{name: "zsh", shell: "zsh", zdotdir: true, wantCommand: "caramel completion zsh"},
		{name: "fish", shell: "fish", xdgConfigHome: true, wantCommand: "caramel completion fish"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			zdotdir := ""
			if test.zdotdir {
				zdotdir = filepath.Join(home, "zsh")
			}
			xdgConfigHome := ""
			if test.xdgConfigHome {
				xdgConfigHome = filepath.Join(home, "config")
			}
			manager := NewCompletionManager(home, test.shell, zdotdir, xdgConfigHome)
			states, err := manager.Status()
			if err != nil || len(states) != 1 || states[0].State != "absent" {
				t.Fatalf("status inicial inesperado: states=%#v err=%v", states, err)
			}

			states, err = manager.Enable()
			if err != nil || len(states) != 1 || states[0].State != "active" {
				t.Fatalf("ativação inesperada: states=%#v err=%v", states, err)
			}
			profile := states[0].Path
			contents, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			text := string(contents)
			if !strings.Contains(text, test.wantCommand) || !strings.Contains(text, "2>/dev/null") {
				t.Fatalf("bloco não contém carregamento dinâmico esperado: %s", text)
			}
			if !strings.Contains(text, "# >>> caramel completion "+test.shell+" >>>") {
				t.Fatalf("marcador de início ausente: %s", text)
			}

			external := "# personalização do usuário\n" + text
			if err := os.WriteFile(profile, []byte(external), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := manager.Enable(); err != nil {
				t.Fatalf("ativação repetida falhou: %v", err)
			}
			updated, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			if string(updated) != external {
				t.Fatalf("ativação repetida alterou o perfil:\nesperava=%q\nrecebeu=%q", external, updated)
			}

			states, err = manager.Disable()
			if err != nil || states[0].State != "not_configured" {
				t.Fatalf("desativação inesperada: states=%#v err=%v", states, err)
			}
			remaining, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			if string(remaining) != "# personalização do usuário\n" {
				t.Fatalf("conteúdo externo alterado: %q", remaining)
			}
			if _, err := manager.Disable(); err != nil {
				t.Fatalf("segunda desativação deveria ser segura: %v", err)
			}
		})
	}
}

func TestCompletionManagerStatusDistinguishesMalformedProfile(t *testing.T) {
	home := t.TempDir()
	manager := NewCompletionManager(home, "bash", "", "")
	profile, err := manager.profiles.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(profile.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	malformed := "# >>> caramel completion bash >>>\n"
	if err := os.WriteFile(profile.Path, []byte(malformed), 0o600); err != nil {
		t.Fatal(err)
	}
	states, err := manager.Status()
	if err != nil || len(states) != 1 || states[0].State != "invalid" {
		t.Fatalf("status inválido inesperado: states=%#v err=%v", states, err)
	}
	if _, err := manager.Enable(); err == nil {
		t.Fatal("ativação deveria rejeitar marcadores inválidos")
	}
	contents, err := os.ReadFile(profile.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != malformed {
		t.Fatalf("perfil inválido foi alterado: %q", contents)
	}
}

func TestCompletionManagerRejectsUnsupportedShell(t *testing.T) {
	manager := NewCompletionManager(t.TempDir(), "tcsh", "", "")
	if _, err := manager.Status(); err == nil || !strings.Contains(err.Error(), "bash, zsh ou fish") {
		t.Fatalf("erro inesperado: %v", err)
	}
}
