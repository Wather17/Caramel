package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestShorthandsLegadosPermanecemEstaveis(t *testing.T) {
	tests := []struct {
		path      []string
		flagName  string
		shorthand string
	}{
		{path: []string{"image", "generate"}, flagName: "items", shorthand: "i"},
		{path: []string{"print", "2up"}, flagName: "margin", shorthand: "m"},
		{path: []string{"image", "colorize"}, flagName: "interactive", shorthand: "i"},
		{path: []string{"image", "colorize"}, flagName: "model", shorthand: "m"},
	}

	for _, tt := range tests {
		cmd, _, err := RootCmd.Find(tt.path)
		if err != nil || cmd == nil {
			t.Fatalf("comando %v não encontrado: %v", tt.path, err)
		}
		flag := cmd.Flags().Lookup(tt.flagName)
		if flag == nil {
			t.Errorf("flag --%s não encontrada em %v", tt.flagName, tt.path)
			continue
		}
		if flag.Shorthand != tt.shorthand {
			t.Errorf("flag --%s em %v usa -%s; esperado -%s", tt.flagName, tt.path, flag.Shorthand, tt.shorthand)
		}
	}
}

func TestCadaComandoNaoDuplicaShorthandLocal(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		seen := map[string]string{}
		cmd.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Shorthand == "" {
				return
			}
			if previous, ok := seen[flag.Shorthand]; ok {
				t.Errorf("comando %s reutiliza -%s em --%s e --%s", cmd.CommandPath(), flag.Shorthand, previous, flag.Name)
			} else {
				seen[flag.Shorthand] = flag.Name
			}
		})
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}

	walk(RootCmd)
}
