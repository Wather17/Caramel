package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/output"

	"github.com/spf13/cobra"
)

func TestShellCommandsManagePowerShellProfiles(t *testing.T) {
	home := fakeWindowsShellRuntime(t)

	if result := runShellCommandForTest(t, shellStatusCmd, []string{"powershell"}); !strings.Contains(result, "0/2 perfil(is) ativo(s)") {
		t.Fatalf("status inicial inesperado: %q", result)
	}
	if result := runShellCommandForTest(t, shellEnableCmd, []string{"powershell"}); !strings.Contains(result, "2/2 perfil(is) ativo(s)") {
		t.Fatalf("ativação inesperada: %q", result)
	}
	if result := runShellCommandForTest(t, shellDisableCmd, []string{"powershell"}); !strings.Contains(result, "0/2 perfil(is) ativo(s)") {
		t.Fatalf("remoção inesperada: %q", result)
	}

	profile := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	contents, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "caramel powershell completion") {
		t.Fatalf("bloco ainda presente após remoção: %s", contents)
	}
}

func TestShellCommandsRejectUnsupportedShellAndPlatform(t *testing.T) {
	fakeWindowsShellRuntime(t)
	if err := shellStatusCmd.RunE(shellStatusCmd, []string{"zsh"}); err == nil || !strings.Contains(err.Error(), "shell não suportado") {
		t.Fatalf("erro para shell desconhecido: %v", err)
	}
	previousOS := currentOperatingSystem
	currentOperatingSystem = "linux"
	t.Cleanup(func() { currentOperatingSystem = previousOS })
	if err := shellStatusCmd.RunE(shellStatusCmd, []string{"powershell"}); err == nil || !strings.Contains(err.Error(), "apenas no Windows") {
		t.Fatalf("erro para plataforma não suportada: %v", err)
	}
}

func TestVaultInitOffersPowerShellIntegrationWithConsent(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		answer     string
		wantActive bool
	}{
		{name: "recusa", answer: "n\n", wantActive: false},
		{name: "aceita", answer: "s\n", wantActive: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			home := fakeWindowsShellRuntime(t)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
			t.Setenv("CARAMEL_LIBRARY_DIR", "")
			library := filepath.Join(t.TempDir(), "biblioteca")
			var stdout, stderr bytes.Buffer
			vaultInitCmd.SetOut(&stdout)
			vaultInitCmd.SetErr(&stderr)
			vaultInitCmd.SetIn(strings.NewReader(scenario.answer))
			vaultInitCmd.SetContext(context.Background())
			t.Cleanup(func() {
				vaultInitCmd.SetOut(nil)
				vaultInitCmd.SetErr(nil)
				vaultInitCmd.SetIn(nil)
			})
			if err := vaultInitCmd.RunE(vaultInitCmd, []string{library}); err != nil {
				t.Fatalf("vault init falhou: %v (stderr=%s)", err, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Ativar autocomplete do vault no PowerShell?") {
				t.Fatalf("prompt ausente: %q", stdout.String())
			}
			profile := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
			_, err := os.Stat(profile)
			if scenario.wantActive && err != nil {
				t.Fatalf("perfil deveria estar ativo: %v", err)
			}
			if scenario.wantActive {
				contents, readErr := os.ReadFile(profile)
				if readErr != nil || !strings.Contains(string(contents), "# >>> caramel powershell completion >>>") {
					t.Fatalf("perfil ativo deveria conter o bloco gerenciado: %v\n%s", readErr, contents)
				}
			}
			if !scenario.wantActive && !os.IsNotExist(err) {
				t.Fatalf("recusa não deveria criar perfil: %v", err)
			}
		})
	}
}

func TestVaultInitDoesNotPromptInJSONOrQuietMode(t *testing.T) {
	for _, mode := range []struct {
		name  string
		json  bool
		quiet bool
	}{
		{name: "json", json: true},
		{name: "quiet", quiet: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			fakeWindowsShellRuntime(t)
			previousJSON, previousQuiet := jsonFlag, quietFlag
			jsonFlag, quietFlag = mode.json, mode.quiet
			t.Cleanup(func() { jsonFlag, quietFlag = previousJSON, previousQuiet })
			renderer, err := output.New(outputOptions(), &bytes.Buffer{}, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			command := &cobra.Command{}
			command.SetIn(strings.NewReader("s\n"))
			if shouldOfferPowerShellIntegration(command, renderer) {
				t.Fatalf("modo %s não deveria pedir confirmação", mode.name)
			}
		})
	}
}

func runShellCommandForTest(t *testing.T, cmd *cobra.Command, args []string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetContext(context.Background())
	t.Cleanup(func() {
		cmd.SetOut(nil)
		cmd.SetErr(nil)
	})
	if err := cmd.RunE(cmd, args); err != nil {
		t.Fatalf("%s falhou: %v (stderr=%s)", cmd.CommandPath(), err, stderr.String())
	}
	return stdout.String()
}

func fakeWindowsShellRuntime(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	previousOS := currentOperatingSystem
	previousHome := currentUserHomeDir
	previousInteractive := interactiveInput
	currentOperatingSystem = "windows"
	currentUserHomeDir = func() (string, error) { return home, nil }
	interactiveInput = func(reader io.Reader) bool { return true }
	t.Cleanup(func() {
		currentOperatingSystem = previousOS
		currentUserHomeDir = previousHome
		interactiveInput = previousInteractive
	})
	return home
}
