package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"caramel/internal/output"
	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

var (
	usageDays    int
	usageAllTime bool
	usageCSV     bool
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Mostra estatísticas locais de uso dos comandos e flags",
	Long: `Resume quantas vezes os comandos e flags foram usados nesta instalação. Registra somente
caminhos de comandos e nomes de flags, sem valores ou argumentos.

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
		report, err := store.CLIUsage(cmd.Context(), since)
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
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Uso local dos comandos e flags (%s)\nTotal de execuções: %d\n\nComandos\n", period, report.Total); err != nil {
			return err
		}
		writeUsageTable(cmd.OutOrStdout(), report.Commands)
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nFlags"); err != nil {
			return err
		}
		writeUsageTable(cmd.OutOrStdout(), report.Flags)
		return nil
	},
}

func writeUsageTable(w io.Writer, counts []vault.UsageCount) {
	if len(counts) == 0 {
		_, _ = fmt.Fprintln(w, "  Sem dados no período.")
		return
	}
	for _, item := range counts {
		_, _ = fmt.Fprintf(w, "  %-32s %d\n", item.Name, item.Count)
	}
}

func writeUsageCSV(w io.Writer, report vault.UsageReport, period string) error {
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
	for _, item := range report.Flags {
		if err := writer.Write([]string{"flag", item.Name, fmt.Sprint(item.Count)}); err != nil {
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
