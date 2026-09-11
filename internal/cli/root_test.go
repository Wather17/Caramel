package cli

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const pedagogicalContextMarker = "📚 QUANDO USAR:"

func publicLeafCommands(root *cobra.Command) []*cobra.Command {
	seen := make(map[*cobra.Command]bool)
	var leaves []*cobra.Command
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, cmd := range parent.Commands() {
			if cmd == nil || seen[cmd] || !cmd.IsAvailableCommand() || cmd.Name() == "completion" {
				continue
			}
			seen[cmd] = true
			if cmd.Runnable() && !cmd.HasSubCommands() {
				leaves = append(leaves, cmd)
			}
			walk(cmd)
		}
	}
	walk(root)
	return leaves
}

func aliasViolations(root *cobra.Command) []string {
	var violations []string
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		owners := make(map[string]string)
		for _, cmd := range parent.Commands() {
			if cmd == nil || cmd.Name() == "completion" {
				continue
			}
			if previous, ok := owners[cmd.Name()]; ok {
				violations = append(violations, parent.CommandPath()+": nome "+cmd.Name()+" conflita com "+previous)
			}
			owners[cmd.Name()] = cmd.CommandPath()
			for _, alias := range cmd.Aliases {
				if previous, ok := owners[alias]; ok {
					violations = append(violations, parent.CommandPath()+": alias "+alias+" conflita com "+previous)
				}
				owners[alias] = cmd.CommandPath()
			}
		}
		for _, cmd := range parent.Commands() {
			if cmd != nil && cmd.Name() != "completion" {
				walk(cmd)
			}
		}
	}
	walk(root)
	return violations
}

func TestRootCommandTree(t *testing.T) {
	wantGroups := []string{"docx", "image", "print", "routine", "config", "workspace", "guide", "version"}
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

func TestBuildScriptUsesDevelopmentVersionFromRoot(t *testing.T) {
	contents, err := os.ReadFile("../../scripts/build.sh")
	if err != nil {
		t.Fatalf("não foi possível ler scripts/build.sh: %v", err)
	}

	match := regexp.MustCompile(`VERSION=\$\{1:-"([^"]+)"\}`).FindSubmatch(contents)
	if len(match) != 2 {
		t.Fatal("scripts/build.sh deveria declarar um fallback VERSION explícito")
	}
	if got := string(match[1]); got != Version {
		t.Errorf("fallback do build (%q) diverge da versão de desenvolvimento do root (%q)", got, Version)
	}
}

func TestRootOutputFlagsArePersistent(t *testing.T) {
	for _, name := range []string{"verbose", "quiet", "json"} {
		flag := RootCmd.PersistentFlags().Lookup(name)
		if flag == nil {
			t.Errorf("flag global --%s deveria estar registrada", name)
		}
	}
}

func TestPublicCommandsFollowDocumentationPolicy(t *testing.T) {
	for _, cmd := range publicLeafCommands(RootCmd) {
		cmd := cmd
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			if strings.TrimSpace(cmd.Short) == "" {
				t.Error("comando público precisa de Short")
			}
			if !strings.Contains(cmd.Long, pedagogicalContextMarker) {
				t.Errorf("comando público precisa da seção %q", pedagogicalContextMarker)
			}
			if strings.TrimSpace(cmd.Example) == "" || !strings.Contains(cmd.Example, "caramel ") {
				t.Error("comando público precisa de exemplos executáveis com o prefixo caramel")
			}
		})
	}
}

func TestCommandAliasesDoNotCollideWithSiblings(t *testing.T) {
	if violations := aliasViolations(RootCmd); len(violations) > 0 {
		t.Fatalf("aliases ou nomes de comandos em conflito:\n- %s", strings.Join(violations, "\n- "))
	}

	root := &cobra.Command{Use: "caramel"}
	root.AddCommand(&cobra.Command{Use: "one", Aliases: []string{"same"}})
	root.AddCommand(&cobra.Command{Use: "two", Aliases: []string{"same"}})
	if violations := aliasViolations(root); len(violations) == 0 {
		t.Fatal("a verificação deveria detectar aliases duplicados entre irmãos")
	}
}

func TestBooleanFlagsWithTrueDefaultsHaveOffSwitch(t *testing.T) {
	for _, cmd := range publicLeafCommands(RootCmd) {
		cmd := cmd
		cmd.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Value.Type() != "bool" || flag.DefValue != "true" {
				return
			}
			if off := cmd.LocalNonPersistentFlags().Lookup("no-" + flag.Name); off == nil || off.Value.Type() != "bool" {
				t.Errorf("%s: bool --%s usa true por padrão, mas não possui --no-%s", cmd.CommandPath(), flag.Name, flag.Name)
			}
		})
	}
}
