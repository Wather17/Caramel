package platform

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	missing string
	name    string
	args    []string
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	if name == r.missing {
		return "", errors.New("não encontrado")
	}
	return "resolved/" + name, nil
}

func (r *fakeRunner) Run(name string, args ...string) error {
	r.name = name
	r.args = append([]string(nil), args...)
	return nil
}

func TestWindowsOpenerPassesUnicodePathAsSeparateArgument(t *testing.T) {
	runner := &fakeRunner{}
	opener := NewOpenerFor("windows", runner)
	path := `C:\Users\Ana Maria\resultados\ação final.docx`
	if err := opener.Open(path, false); err != nil {
		t.Fatal(err)
	}
	if runner.name != "resolved/rundll32.exe" || !reflect.DeepEqual(runner.args, []string{"url.dll,FileProtocolHandler", path}) {
		t.Fatalf("argumentos inseguros ou inesperados: %q %#v", runner.name, runner.args)
	}
	if err := opener.Reveal(path, false); err != nil {
		t.Fatal(err)
	}
	if runner.name != "resolved/explorer.exe" || !reflect.DeepEqual(runner.args, []string{"/select,", path}) {
		t.Fatalf("reveal inesperado: %q %#v", runner.name, runner.args)
	}
}

func TestPlatformOpenerUsesNativeBundleCommands(t *testing.T) {
	tests := []struct {
		goos string
		name string
	}{
		{goos: "windows", name: "resolved/explorer.exe"},
		{goos: "darwin", name: "resolved/open"},
		{goos: "linux", name: "resolved/xdg-open"},
	}
	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			runner := &fakeRunner{}
			if err := NewOpenerFor(test.goos, runner).Open("/tmp/pacote", true); err != nil {
				t.Fatal(err)
			}
			if runner.name != test.name || !reflect.DeepEqual(runner.args, []string{"/tmp/pacote"}) {
				t.Fatalf("comando inesperado: %q %#v", runner.name, runner.args)
			}
		})
	}
}

func TestPlatformOpenerReportsMissingApplication(t *testing.T) {
	runner := &fakeRunner{missing: "xdg-open"}
	err := NewOpenerFor("linux", runner).Open("/tmp/resultado.pdf", false)
	if err == nil || !strings.Contains(err.Error(), "xdg-open") || !strings.Contains(err.Error(), "não está disponível") {
		t.Fatalf("erro pouco útil: %v", err)
	}
}
