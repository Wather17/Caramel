package cli

import (
	"strings"
	"testing"
)

func TestRootCommandTree(t *testing.T) {
	wantGroups := []string{"docx", "image", "print", "routine", "config", "guide", "version"}
	registered := make(map[string]bool, len(RootCmd.Commands()))
	for _, sub := range RootCmd.Commands() {
		registered[strings.Fields(sub.Use)[0]] = true
	}

	for _, name := range wantGroups {
		if !registered[name] {
			t.Errorf("comando de primeiro nível %q não está registrado na raiz", name)
		}
	}
}

func TestRootCommandGroupsHaveSubcommands(t *testing.T) {
	for _, name := range []string{"docx", "image", "print", "routine", "config"} {
		sub, _, err := RootCmd.Find([]string{name})
		if err != nil || sub == nil || !sub.HasSubCommands() {
			t.Errorf("grupo %q deveria existir e ter subcomandos", name)
		}
	}
}

func TestRootHelp(t *testing.T) {
	out := captureStdout(t, func() {
		if err := RootCmd.Help(); err != nil {
			t.Errorf("RootCmd.Help() falhou: %v", err)
		}
	})
	if !strings.Contains(out, "caramel") || !strings.Contains(out, "pedagógico") {
		t.Errorf("ajuda da raiz deveria descrever o Caramel, obtido: %s", out)
	}
}

func TestUnknownCommandReturnsError(t *testing.T) {
	RootCmd.SetArgs([]string{"definitely-not-a-command"})
	defer RootCmd.SetArgs(nil)

	err := RootCmd.Execute()
	if err == nil {
		t.Error("comando inexistente deveria retornar erro")
	}
	if !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "desconhecido") {
		t.Errorf("erro deveria indicar comando desconhecido, obtido: %v", err)
	}
}

func TestVersionVariablesAreSet(t *testing.T) {
	if Version == "" || Commit == "" || Date == "" {
		t.Errorf("variáveis de versão devem ter valores padrão, obtido Version=%q Commit=%q Date=%q", Version, Commit, Date)
	}
}
