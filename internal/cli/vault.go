package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/output"
	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

var vaultSyncFull bool

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Configura e sincroniza a biblioteca local do Caramel",
	Long:  "Gerencia a biblioteca visível, suas fontes vinculadas e o índice local do Caramel.",
}

var vaultInitCmd = &cobra.Command{
	Use:   "init [pasta]",
	Short: "Inicializa a biblioteca visível do Caramel",
	Long: `Cria as pastas materiais e resultados e registra ambas no índice, mantendo banco,
cache e objetos internos no diretório privado do aplicativo.

📚 QUANDO USAR:
Use no primeiro uso do vault ou para escolher uma nova biblioteca principal.`,
	Example: `# Usar Documents/Caramel
caramel vault init

# Escolher explicitamente a biblioteca
caramel vault init "C:\Users\55689\Documents\Caramel"`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		root := ""
		if len(args) == 1 {
			root, err = filepath.Abs(strings.TrimSpace(args[0]))
		} else {
			root, err = config.ResolveLibraryDir(cfg)
		}
		if err != nil {
			return err
		}
		root = filepath.Clean(root)
		v, err := vault.Open()
		if err != nil {
			return err
		}
		defer v.Close()
		sources, err := v.InitializeLibrary(cmd.Context(), root)
		if err != nil {
			return err
		}
		if err := config.SaveConfigValue("CARAMEL_LIBRARY_DIR", root); err != nil {
			return err
		}
		report, err := v.Sync(cmd.Context(), false)
		if err != nil {
			return err
		}
		warnings := append([]string(nil), report.Warnings...)
		if err := offerPowerShellIntegration(cmd, renderer); err != nil {
			warnings = append(warnings, fmt.Sprintf("integração do PowerShell não foi ativada: %v", err))
		}
		return renderer.Result(output.Result{
			Status:   stateForWarnings(warnings),
			Summary:  fmt.Sprintf("Biblioteca inicializada com %d fonte(s).", len(sources)),
			Outputs:  []string{root},
			Data:     report,
			Warnings: warnings,
		})
	},
}

var vaultSourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Gerencia pastas vinculadas ao índice",
	Long:  "Adiciona e lista fontes externas sem copiar, mover ou renomear seus arquivos.",
}

var vaultSourceAddCmd = &cobra.Command{
	Use:   "add <pasta>",
	Short: "Adiciona uma pasta externa ao índice",
	Long: `Registra uma pasta externa para sincronização recursiva, preservando integralmente
seus arquivos e sua organização atual.

📚 QUANDO USAR:
Use para incluir um acervo existente, como Documents/docs-mae, sem migrá-lo.`,
	Example: `# Vincular um acervo existente
caramel vault source add "C:\Users\55689\Documents\docs-mãe"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		v, err := vault.Open()
		if err != nil {
			return err
		}
		defer v.Close()
		source, err := v.AddSource(cmd.Context(), args[0], vault.SourceExternal)
		if err != nil {
			return err
		}
		return renderer.Result(output.Result{Status: output.StateSuccess, Summary: "Fonte externa adicionada ao índice.", Outputs: []string{source.Path}, Data: source})
	},
}

var vaultSourceListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista as fontes da biblioteca",
	Long: `Exibe cada raiz vinculada, seu papel e a última sincronização concluída.

📚 QUANDO USAR:
Use para confirmar quais pastas alimentam o autocomplete e a busca do vault.`,
	Example: `# Listar fontes configuradas
caramel vault source list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		v, err := vault.Open()
		if err != nil {
			return err
		}
		defer v.Close()
		sources, err := v.ListSources(cmd.Context())
		if err != nil {
			return err
		}
		for _, source := range sources {
			last := "nunca"
			if source.LastScannedAt != nil {
				last = source.LastScannedAt.Local().Format("02/01/2006 15:04:05")
			}
			if err := renderer.Text("%-10s %s · última sincronização: %s\n", source.Role, source.Path, last); err != nil {
				return err
			}
		}
		return renderer.Result(output.Result{Status: output.StateSuccess, Summary: fmt.Sprintf("Fontes configuradas: %d.", len(sources)), Count: len(sources), Data: sources})
	},
}

var vaultSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sincroniza os metadados das fontes",
	Long: `Percorre as fontes recursivamente e atualiza somente arquivos novos ou alterados.
Com --full, recalcula todos os hashes para reparar o índice.

📚 QUANDO USAR:
Use para forçar uma atualização imediata ou diagnosticar uma fonte; o fluxo normal pode sincronizar automaticamente.`,
	Example: `# Sincronização incremental
caramel vault sync

# Recalcular todos os hashes
caramel vault sync --full`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		v, err := vault.Open()
		if err != nil {
			return err
		}
		defer v.Close()
		report, err := v.Sync(cmd.Context(), vaultSyncFull)
		if err != nil {
			return err
		}
		return renderer.Result(output.Result{
			Status:   stateForWarnings(report.Warnings),
			Summary:  fmt.Sprintf("Índice sincronizado: %d arquivo(s), %d novo(s), %d alterado(s), %d movido(s).", report.Files, report.Created, report.Updated, report.Moved),
			Count:    report.Files,
			Data:     report,
			Warnings: report.Warnings,
		})
	},
}

var vaultStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostra o estado da biblioteca e do índice",
	Long: `Exibe a biblioteca principal, fontes, quantidades e datas da última sincronização sem
percorrer novamente o sistema de arquivos.

📚 QUANDO USAR:
Use para diagnosticar configuração, fontes indisponíveis ou um índice desatualizado.`,
	Example: `# Consultar o índice sem sincronizar
caramel vault status`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		libraryDir, err := config.ResolveLibraryDir(cfg)
		if err != nil {
			return err
		}
		v, err := vault.Open()
		if err != nil {
			return err
		}
		defer v.Close()
		status, err := v.Status(cmd.Context())
		if err != nil {
			return err
		}
		sources, err := v.ListSources(cmd.Context())
		if err != nil {
			return err
		}
		data := struct {
			Library string                `json:"library"`
			Index   vault.IndexStatus     `json:"index"`
			Sources []vault.IndexedSource `json:"sources"`
		}{Library: libraryDir, Index: status, Sources: sources}
		for _, source := range sources {
			last := "nunca"
			if source.LastScannedAt != nil {
				last = source.LastScannedAt.Local().Format("02/01/2006 15:04:05")
			}
			if err := renderer.Text("%-10s %s · última sincronização: %s\n", source.Role, source.Path, last); err != nil {
				return err
			}
		}
		return renderer.Result(output.Result{Status: output.StateSuccess, Summary: fmt.Sprintf("Vault: %d fonte(s), %d arquivo(s) disponível(is), %d indisponível(is).", status.Sources, status.Available, status.Unavailable), Data: data, Outputs: []string{libraryDir}})
	},
}

func stateForWarnings(warnings []string) output.State {
	if len(warnings) > 0 {
		return output.StateWarning
	}
	return output.StateSuccess
}

func init() {
	vaultSyncCmd.Flags().BoolVar(&vaultSyncFull, "full", false, "Recalcula todos os hashes do índice")
	vaultSourceCmd.AddCommand(vaultSourceAddCmd, vaultSourceListCmd)
	vaultCmd.AddCommand(vaultInitCmd, vaultSourceCmd, vaultSyncCmd, vaultStatusCmd)
	RootCmd.AddCommand(vaultCmd)
}
