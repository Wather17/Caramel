package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/output"
	"caramel/internal/platform"
	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

var resultOpener platform.Opener = platform.NewOpener()

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Abre resultados do Caramel no aplicativo padrão",
	Long:  "Abre resultados publicados pelo Caramel no aplicativo padrão do sistema.",
}

var openLastCmd = &cobra.Command{
	Use:   "last",
	Short: "Abre o último resultado publicado",
	Long: `Consulta o vault e abre o resultado concluído mais recente. Arquivos únicos usam o
aplicativo padrão; pacotes abrem no gerenciador de arquivos.

📚 QUANDO USAR:
Use logo após produzir um material para revisá-lo sem procurar manualmente na pasta de resultados.`,
	Example: `# Abrir o arquivo ou pacote produzido por último
caramel open last`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLastResultAction(cmd, false)
	},
}

var revealCmd = &cobra.Command{
	Use:   "reveal",
	Short: "Mostra resultados do Caramel no gerenciador de arquivos",
	Long:  "Revela resultados publicados pelo Caramel no gerenciador de arquivos do sistema.",
}

var revealLastCmd = &cobra.Command{
	Use:   "last",
	Short: "Revela o último resultado no gerenciador de arquivos",
	Long: `Consulta o vault e mostra o resultado concluído mais recente no gerenciador de arquivos.
No Windows, arquivos únicos são selecionados no Explorer; pacotes têm sua pasta aberta.

📚 QUANDO USAR:
Use para localizar, renomear, copiar ou compartilhar o material que o Caramel acabou de produzir.`,
	Example: `# Selecionar o último arquivo ou abrir a pasta do último pacote
caramel reveal last`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLastResultAction(cmd, true)
	},
}

func runLastResultAction(cmd *cobra.Command, reveal bool) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	store, err := vault.Open()
	if err != nil {
		return fmt.Errorf("não foi possível abrir o vault: %w", err)
	}
	defer store.Close()
	result, err := store.LastPathResult(ctx)
	if errors.Is(err, vault.ErrNoPathResult) {
		return fmt.Errorf("nenhum resultado concluído foi encontrado; produza um arquivo antes de usar este comando")
	}
	if err != nil {
		return fmt.Errorf("não foi possível consultar o último resultado: %w", err)
	}
	info, err := os.Stat(result.PrimaryPath)
	if err != nil {
		return unavailableResultError(result, err)
	}
	bundle := result.Shape == vault.PathResultBundle || info.IsDir()
	if reveal {
		err = resultOpener.Reveal(result.PrimaryPath, bundle)
	} else {
		err = resultOpener.Open(result.PrimaryPath, bundle)
	}
	if err != nil {
		return fmt.Errorf("não foi possível acessar o resultado da run %s: %w", result.RunID, err)
	}
	renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	action := "aberto"
	if reveal {
		action = "revelado"
	}
	return renderer.Result(output.Result{
		Status:        output.StateSuccess,
		Summary:       fmt.Sprintf("Último resultado %s (run %s).", action, result.RunID),
		Outputs:       []string{result.PrimaryPath},
		PrimaryOutput: result.PrimaryPath,
		Data:          result,
	})
}

func unavailableResultError(result vault.PathResult, cause error) error {
	return fmt.Errorf("resultado da run %s indisponível em %q: %v; execute 'caramel vault sync' para atualizar o índice", result.RunID, result.PrimaryPath, cause)
}

func openPublishedResult(result *output.Result) string {
	primary, shape := describePublishedResult(result)
	if primary == "" {
		return ""
	}
	if _, err := os.Stat(primary); err != nil {
		return fmt.Sprintf("output publicado, mas não foi possível abri-lo em %q: %v", primary, err)
	}
	if err := resultOpener.Open(primary, shape == vault.PathResultBundle); err != nil {
		return fmt.Sprintf("output publicado, mas não foi possível abri-lo: %v", err)
	}
	return ""
}

func describePublishedResult(result *output.Result) (string, vault.PathResultShape) {
	if result == nil {
		return "", ""
	}
	primary := strings.TrimSpace(result.PrimaryOutput)
	outputs := nonEmptyPaths(result.Outputs)
	if primary == "" {
		switch len(outputs) {
		case 0:
			return "", ""
		case 1:
			primary = outputs[0]
		default:
			primary = commonOutputDirectory(outputs)
		}
	}
	absolute, err := filepath.Abs(primary)
	if err == nil {
		primary = filepath.Clean(absolute)
	} else {
		primary = filepath.Clean(primary)
	}
	shape := vault.PathResultSingle
	if len(outputs) > 1 {
		shape = vault.PathResultBundle
	}
	if info, err := os.Stat(primary); err == nil && info.IsDir() {
		shape = vault.PathResultBundle
	}
	return primary, shape
}

func nonEmptyPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) != "" {
			result = append(result, filepath.Clean(path))
		}
	}
	return result
}

func commonOutputDirectory(paths []string) string {
	common := filepath.Dir(paths[0])
	for _, path := range paths[1:] {
		directory := filepath.Dir(path)
		for directory != common && !strings.HasPrefix(directory+string(filepath.Separator), common+string(filepath.Separator)) {
			parent := filepath.Dir(common)
			if parent == common {
				return filepath.Dir(paths[0])
			}
			common = parent
		}
	}
	return common
}

func openRequested(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Flags().Lookup("open") == nil {
		return false
	}
	value, err := cmd.Flags().GetBool("open")
	return err == nil && value
}

func registerOpenFlag(cmd *cobra.Command) {
	if cmd.Flags().Lookup("open") == nil {
		cmd.Flags().Bool("open", false, "Abre o resultado principal após a publicação")
	}
}

func init() {
	for _, command := range []*cobra.Command{
		docxExtractCmd, docxImagesExtractCmd, docxSplitCmd, docxMergeCmd,
		imageColorizeCmd, imageGenerateCmd, cardsCmd, pdf2UpCmd,
		pdfCreateCmd, pdfMergeCmd, pdfSplitCmd, pdfPagesRenderCmd, pdfImagesExtractCmd,
		routineProcessCmd, routineConsolidateCmd,
	} {
		registerOpenFlag(command)
	}
	openCmd.AddCommand(openLastCmd)
	revealCmd.AddCommand(revealLastCmd)
	RootCmd.AddCommand(openCmd, revealCmd)
}
