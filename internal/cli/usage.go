package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"time"

	"caramel/internal/output"
	"caramel/internal/vault"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	usageDays    int
	usageAllTime bool
	usageCSV     bool
)

var usageCompatibilityPaths = map[string]string{
	"caramel 2up":             "caramel print 2up",
	"caramel cards":           "caramel print cards",
	"caramel colorize":        "caramel image colorize",
	"caramel generate":        "caramel image generate",
	"caramel process":         "caramel image colorize",
	"caramel pipeline":        "caramel image colorize",
	"caramel run":             "caramel image colorize",
	"caramel docx extract":    "caramel docx images extract",
	"caramel routine process": "caramel routine consolidate",
	"caramel config models":   "caramel config models select",
}

func canonicalUsagePath(path string) string {
	if canonical, ok := usageCompatibilityPaths[path]; ok {
		return canonical
	}
	return path
}

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Mostra estatísticas locais de uso dos comandos e flags",
	Long: `Resume adoção, saúde, tendência e sinais locais dos comandos e flags desta instalação.
Registra somente caminhos canônicos, nomes de flags, datas, durações e resultados, sem valores,
argumentos ou caminhos de arquivos.

📚 QUANDO USAR:
Use para entender quais partes do Caramel fazem parte do seu fluxo diário. Frequência baixa é
apenas um dado para análise e não significa que um comando seja desnecessário.`,
	Example: `# Consultar os últimos 30 dias
caramel usage

# Ver todo o histórico local
caramel usage --all-time

# Exportar em CSV
caramel usage --days 90 --csv`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if usageDays < 1 {
			return fmt.Errorf("--days deve ser maior que zero")
		}
		if usageAllTime && cmd.Flags().Changed("days") {
			return fmt.Errorf("--all-time não pode ser usada com --days")
		}
		if usageCSV && (jsonFlag || quietFlag) {
			return fmt.Errorf("--csv não pode ser usada com --json ou --quiet")
		}
		var since *time.Time
		period := fmt.Sprintf("últimos %d dias", usageDays)
		if usageAllTime {
			period = "todo o histórico"
		} else {
			value := time.Now().UTC().AddDate(0, 0, -usageDays)
			since = &value
		}
		store, err := vault.Open()
		if err != nil {
			return err
		}
		defer store.Close()
		report, err := store.DetailedCLIUsage(cmd.Context(), since, usageCatalog(RootCmd))
		if err != nil {
			return err
		}
		if usageCSV {
			return writeUsageCSV(cmd.OutOrStdout(), report, period)
		}
		if jsonFlag {
			renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			return renderer.Result(output.Result{Status: output.StateSuccess, Summary: "Relatório local de uso: " + period + ".", Count: report.Total, Data: report})
		}
		return writeDetailedUsageHuman(cmd.OutOrStdout(), report, period)
	},
}

func visitUsageFlags(cmd *cobra.Command, visit func(*pflag.Flag)) {
	seen := make(map[string]struct{})
	visitSet := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if _, ok := seen[flag.Name]; ok || flag.Hidden || flag.Name == "help" {
				return
			}
			seen[flag.Name] = struct{}{}
			visit(flag)
		})
	}
	visitSet(cmd.InheritedFlags())
	visitSet(cmd.PersistentFlags())
	visitSet(cmd.LocalNonPersistentFlags())
}

func usageCatalog(root *cobra.Command) vault.UsageCatalog {
	var catalog vault.UsageCatalog
	seen := make(map[*cobra.Command]bool)
	addLeaf := func(cmd *cobra.Command, path string) {
		catalog.Commands = append(catalog.Commands, path)
		visitUsageFlags(cmd, func(flag *pflag.Flag) {
			catalog.Flags = append(catalog.Flags, vault.UsageFlagDefinition{CommandPath: path, Name: flag.Name})
		})
	}
	var walkGroup func(*cobra.Command, string)
	walkGroup = func(group *cobra.Command, groupPath string) {
		for _, cmd := range group.Commands() {
			if cmd == nil || seen[cmd] || cmd.Name() == "usage" || cmd.Name() == "completion" || cmd.Hidden || !cmd.IsAvailableCommand() {
				continue
			}
			seen[cmd] = true
			path := groupPath + " " + cmd.Name()
			if cmd.Runnable() && !cmd.HasSubCommands() {
				addLeaf(cmd, path)
			}
			walkGroup(cmd, path)
		}
	}

	// Primeiro, percorra os grupos canônicos. Isso evita que registros de
	// compatibilidade na raiz substituam o caminho pedagógico agrupado.
	for _, child := range root.Commands() {
		if child == nil || seen[child] || child.Runnable() || child.Name() == "completion" || child.Hidden || !child.IsAvailableCommand() {
			continue
		}
		seen[child] = true
		walkGroup(child, "caramel "+child.Name())
	}

	// Depois, inclua somente comandos autênticos da raiz.
	for _, child := range root.Commands() {
		if child == nil || seen[child] || !child.Runnable() || child.Name() == "usage" || child.Name() == "completion" || child.Hidden || !child.IsAvailableCommand() {
			continue
		}
		seen[child] = true
		addLeaf(child, "caramel "+child.Name())
	}
	sort.Strings(catalog.Commands)
	sort.Slice(catalog.Flags, func(i, j int) bool {
		if catalog.Flags[i].CommandPath == catalog.Flags[j].CommandPath {
			return catalog.Flags[i].Name < catalog.Flags[j].Name
		}
		return catalog.Flags[i].CommandPath < catalog.Flags[j].CommandPath
	})
	return catalog
}

func writeDetailedUsageHuman(w io.Writer, report vault.DetailedUsageReport, period string) error {
	if _, err := fmt.Fprintf(w, "Uso local dos comandos e flags (%s)\nTotal de execuções: %d\n\nCobertura da evidência\n", period, report.Total); err != nil {
		return err
	}
	coverage := report.Coverage
	if _, err := fmt.Fprintf(w, "  Dados disponíveis: %t\n  Eventos: %d\n  Dias ativos: %d\n  Dias observados: %d\n", coverage.HasData, coverage.Events, coverage.ActiveDays, coverage.ObservedDays); err != nil {
		return err
	}
	if coverage.FirstCollectedAt != nil {
		if _, err := fmt.Fprintf(w, "  Primeira coleta: %s\n", formatUsageTime(coverage.FirstCollectedAt)); err != nil {
			return err
		}
	}
	if coverage.LastCollectedAt != nil {
		if _, err := fmt.Fprintf(w, "  Última coleta: %s\n", formatUsageTime(coverage.LastCollectedAt)); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "\nComandos"); err != nil {
		return err
	}
	for _, command := range report.Commands {
		average := "n/d"
		if command.DurationSamples > 0 {
			average = fmt.Sprintf("%d ms", command.AverageDurationMS)
		}
		lastUsed := "nunca"
		if command.LastUsed != nil {
			lastUsed = formatUsageTime(command.LastUsed)
		}
		if _, err := fmt.Fprintf(w, "  %-36s %d (%.0f%%) | ativos: %d | última: %s | média: %s\n", command.Name, command.Count, command.Share*100, command.ActiveDays, lastUsed, average); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "    status: sucesso=%d aviso=%d falha=%d cancelado=%d desconhecido=%d\n", command.Status.Success, command.Status.Warning, command.Status.Failed, command.Status.Canceled, command.Status.Unknown); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "\nFlags"); err != nil {
		return err
	}
	for _, flag := range report.Flags {
		label := fmt.Sprintf("%s --%s", flag.CommandPath, flag.Name)
		lastUsed := "nunca"
		if flag.LastUsed != nil {
			lastUsed = formatUsageTime(flag.LastUsed)
		}
		if _, err := fmt.Fprintf(w, "  %-52s %d | última: %s\n", label, flag.Count, lastUsed); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "\nTendência semanal"); err != nil {
		return err
	}
	if len(report.Trend) == 0 {
		if _, err := fmt.Fprintln(w, "  Sem dados no período."); err != nil {
			return err
		}
	} else {
		for _, bucket := range report.Trend {
			if _, err := fmt.Fprintf(w, "  %s a %s: %d execução(ões)\n", bucket.Start.Format("2006-01-02"), bucket.End.Format("2006-01-02"), bucket.Count); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(w, "\nSinais para revisão humana"); err != nil {
		return err
	}
	if len(report.Signals) == 0 {
		_, err := fmt.Fprintln(w, "  Nenhum sinal observado.")
		return err
	}
	for _, signal := range report.Signals {
		target := signal.CommandPath
		if signal.Flag != "" {
			target += " --" + signal.Flag
		}
		if target != "" {
			target = " " + target
		}
		if _, err := fmt.Fprintf(w, "  [%s]%s: %s\n", signal.Kind, target, signal.Detail); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "\nA frequência é evidência para análise; não determina remoção ou simplificação automaticamente.")
	return err
}

func formatUsageTime(value *time.Time) string {
	if value == nil {
		return "nunca"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func writeUsageCSV(w io.Writer, report vault.DetailedUsageReport, period string) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"tipo", "nome", "contagem"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"período", period, ""}); err != nil {
		return err
	}
	for _, item := range report.Commands {
		if err := writer.Write([]string{"comando", item.Name, fmt.Sprint(item.Count)}); err != nil {
			return err
		}
	}
	flagTotals := make(map[string]int)
	for _, item := range report.Flags {
		flagTotals[item.Name] += item.Count
	}
	flagNames := make([]string, 0, len(flagTotals))
	for name := range flagTotals {
		flagNames = append(flagNames, name)
	}
	sort.Strings(flagNames)
	for _, name := range flagNames {
		if err := writer.Write([]string{"flag", name, fmt.Sprint(flagTotals[name])}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func init() {
	usageCmd.Flags().IntVar(&usageDays, "days", 30, "Considera os últimos N dias")
	usageCmd.Flags().BoolVar(&usageAllTime, "all-time", false, "Considera todo o histórico local")
	usageCmd.Flags().BoolVar(&usageCSV, "csv", false, "Exporta o resumo agregado em CSV")
	RootCmd.AddCommand(usageCmd)
}
