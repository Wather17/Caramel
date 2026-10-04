package shell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPowerShellManagerEnableUpdateAndDisablePreservesProfile(t *testing.T) {
	home := t.TempDir()
	manager := NewPowerShellManager(home, "windows")
	profiles, err := manager.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	original := "function prompt { 'caramel' }\r\n"
	if err := os.MkdirAll(filepath.Dir(profiles[0].Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profiles[0].Path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	states, err := manager.Enable()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states[0].State != "active" || states[1].State != "active" {
		t.Fatalf("estado após ativação: %#v", states)
	}
	first, err := os.ReadFile(profiles[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	contents := string(first)
	for _, want := range []string{
		original,
		powerShellBlockStart,
		"[Console]::OutputEncoding = $caramelUtf8Encoding",
		"caramel completion powershell | Out-String | Invoke-Expression",
		"$_ -match '[\\s\\\\]'",
		"$quote + $_.Replace($quote, $quote + $quote) + $quote",
		powerShellBlockEnd,
	} {
		if !strings.Contains(contents, want) {
			t.Fatalf("perfil deveria conter %q:\n%s", want, contents)
		}
	}
	if !strings.Contains(contents, "\r\n"+powerShellBlockStart) {
		t.Fatalf("perfil deveria preservar CRLF: %q", contents)
	}

	if _, err := manager.Enable(); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(profiles[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != contents {
		t.Fatalf("segunda ativação não foi idempotente:\nantes=%q\ndepois=%q", contents, second)
	}

	states, err = manager.Disable()
	if err != nil {
		t.Fatal(err)
	}
	if states[0].State != "not_configured" || states[1].State != "not_configured" {
		t.Fatalf("estado após desativação: %#v", states)
	}
	remaining, err := os.ReadFile(profiles[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(remaining) != original {
		t.Fatalf("conteúdo externo alterado:\nesperava=%q\nrecebeu=%q", original, remaining)
	}
	if _, err := manager.Disable(); err != nil {
		t.Fatalf("segunda desativação deveria ser segura: %v", err)
	}
}

func TestPowerShellManagerRejectsMalformedMarkers(t *testing.T) {
	manager := NewPowerShellManager(t.TempDir(), "windows")
	profiles, err := manager.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(profiles[0].Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profiles[0].Path, []byte(powerShellBlockStart+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Enable(); err == nil || !strings.Contains(err.Error(), "marcadores") {
		t.Fatalf("esperava erro de marcador inválido, recebeu %v", err)
	}
}

func TestPowerShellManagerRejectsNonWindows(t *testing.T) {
	manager := NewPowerShellManager(t.TempDir(), "linux")
	if _, err := manager.Status(); !errors.Is(err, ErrPowerShellUnsupported) {
		t.Fatalf("erro=%v, esperava %v", err, ErrPowerShellUnsupported)
	}
}
