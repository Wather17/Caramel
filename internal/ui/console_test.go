package ui

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseConsoleInput(t *testing.T) {
	args, err := parseConsoleInput(`import "./Minha Pasta"`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"import", "./Minha Pasta"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("argumentos inesperados: got=%v want=%v", args, want)
	}

	args, err = parseConsoleInput(`generate animais\, frutas`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"generate", "animais,", "frutas"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("escape inesperado: got=%v want=%v", args, want)
	}
}

func TestParseConsoleInputRejectsUnclosedSyntax(t *testing.T) {
	for _, input := range []string{`import "Downloads`, `import Downloads\`} {
		if _, err := parseConsoleInput(input); err == nil {
			t.Fatalf("esperava erro para %q", input)
		}
	}
}

func TestResolveWorkspaceConsoleCommand(t *testing.T) {
	for _, name := range []string{"help", "find", "i", "g", "c", "f", "p", "exit"} {
		if _, ok := resolveWorkspaceConsoleCommand(name); !ok {
			t.Fatalf("comando/alias não resolvido: %s", name)
		}
	}
	if _, ok := resolveWorkspaceConsoleCommand("rm"); ok {
		t.Fatal("comando de shell não deveria ser resolvido")
	}
}

func TestWorkspaceConsoleHelpLines(t *testing.T) {
	help := strings.Join(workspaceConsoleHelpLines(), "\n")
	for _, expected := range []string{"search [termo]", "import <arquivo ou pasta>", "generate <itens>", "colorize", "2up"} {
		if !strings.Contains(help, expected) {
			t.Fatalf("help não contém %q: %s", expected, help)
		}
	}
}
