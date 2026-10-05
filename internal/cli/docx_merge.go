package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"caramel/internal/output"
	"caramel/internal/platform"
	"caramel/internal/tools/docx"
	"caramel/internal/ui"
	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

var (
	docxMergeOutput string
	docxMergePick   bool
	docxMergeOpener platform.Opener = platform.NewOpener()
	docxMergePicker                 = pickDOCXMergeFiles
)

var docxMergeCmd = &cobra.Command{
	Use:   "merge [arquivo1.docx arquivo2.docx ...]",
	Short: "Junta DOCX preservando ordem, estilos, listas e imagens comuns",
	Long: `Combina dois ou mais arquivos DOCX locais na ordem informada e inicia cada origem em uma nova seção/página.
Preserva texto, tabelas, imagens, estilos, listas e configurações comuns de página. Sem biblioteca configurada, o destino é obrigatório; resultados nunca são sobrescritos.
Comentários, notas de rodapé/fim, revisões controladas e objetos incorporados não são suportados.

Sem argumentos em um terminal interativo, abre um seletor com busca incremental dos DOCX indexados pelo vault. Use --pick para solicitar o seletor explicitamente. No seletor, Ctrl+O revela o arquivo destacado no gerenciador de arquivos e encerra sem executar a junção.

📚 QUANDO USAR:
Use para reunir documentos Word em uma apostila ou material único sem enviar os arquivos para um serviço externo.`,
	Example: `# Juntar arquivos na ordem informada
caramel docx merge capa.docx atividades.docx respostas.docx --output apostila.docx

# Usar a forma curta da flag
caramel docx merge parte-1.docx parte-2.docx -o documento-completo.docx

# Escolher documentos indexados no vault
caramel docx merge --pick`,
	Args: validateDOCXMergeArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptionsFor(cmd), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if len(args) == 0 || docxMergePick {
			if !docxMergePickerAllowed(cmd) {
				return fmt.Errorf("a seleção interativa requer um terminal interativo sem --json ou --quiet")
			}
			picked, pickErr := docxMergePicker(cmd)
			if pickErr != nil {
				return pickErr
			}
			switch picked.Action {
			case ui.FilePickerCanceled:
				return nil
			case ui.FilePickerReveal:
				if err := docxMergeOpener.Reveal(picked.RevealPath, false); err != nil {
					return err
				}
				return renderer.Text("Arquivo revelado: %s\n", picked.RevealPath)
			case ui.FilePickerConfirmed:
				args = picked.Selected
				touchIndexedInputs(cmd, args)
			}
		}
		targetPath := docxMergeOutput
		var plan *output.Plan
		if !cmd.Flags().Changed("output") || strings.TrimSpace(targetPath) == "" {
			plan, err = libraryOutputPlan(cmd, "docx", output.ShapeSingle, outputStem(args[0], "_merged"), ".docx")
			if err != nil {
				return err
			}
			if plan == nil {
				return fmt.Errorf("a flag --output é obrigatória quando a biblioteca não está configurada")
			}
			defer plan.Cleanup()
			targetPath = plan.WorkPath()
		}
		if err := docx.MergeDOCX(args, targetPath); err != nil {
			return err
		}
		paths, err := publishOutputPlan(plan, []string{targetPath})
		if err != nil {
			return err
		}
		return renderer.Result(output.Result{
			Status:  output.StateSuccess,
			Summary: fmt.Sprintf("DOCX juntados: %d arquivo(s).", len(args)),
			Count:   len(args),
			Outputs: paths,
		})
	},
}

func init() {
	docxMergeCmd.Flags().StringVarP(&docxMergeOutput, "output", "o", "", "Caminho do DOCX final")
	docxMergeCmd.Flags().BoolVar(&docxMergePick, "pick", false, "Seleciona os DOCX em uma TUI interativa")
	docxCmd.AddCommand(docxMergeCmd)
}

func validateDOCXMergeArgs(cmd *cobra.Command, args []string) error {
	if docxMergePick {
		if len(args) > 0 {
			return fmt.Errorf("--pick não pode ser combinado com argumentos de arquivo")
		}
		if !docxMergePickerAllowed(cmd) {
			return fmt.Errorf("--pick requer um terminal interativo sem --json ou --quiet")
		}
		return nil
	}
	if len(args) == 0 && docxMergePickerAllowed(cmd) {
		return nil
	}
	return cobra.MinimumNArgs(2)(cmd, args)
}

func docxMergePickerAllowed(cmd *cobra.Command) bool {
	options := outputOptionsFor(cmd)
	return !options.JSON && !options.Quiet && interactiveInput(cmd.InOrStdin())
}

func pickDOCXMergeFiles(cmd *cobra.Command) (ui.FilePickerResult, error) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	store, err := vault.Open()
	if err != nil {
		return ui.FilePickerResult{}, fmt.Errorf("abrir vault para selecionar DOCX: %w", err)
	}
	defer store.Close()
	if _, _, err := store.SyncIfStale(ctx, 30*time.Second); err != nil {
		return ui.FilePickerResult{}, fmt.Errorf("sincronizar índice de DOCX: %w", err)
	}
	candidates, err := store.SearchIndexedFiles(ctx, vault.IndexedFileQuery{Extensions: []string{".docx"}})
	if err != nil {
		return ui.FilePickerResult{}, fmt.Errorf("buscar DOCX indexados: %w", err)
	}
	items := make([]ui.FilePickerItem, 0, len(candidates))
	for _, candidate := range candidates {
		name := candidate.Name
		if strings.TrimSpace(name) == "" {
			name = filepath.Base(candidate.Path)
		}
		items = append(items, ui.FilePickerItem{
			Path:        candidate.Path,
			Name:        name,
			Description: fmt.Sprintf("%s · %s", sourceRoleLabel(candidate.SourceRole), candidate.Path),
		})
	}
	if len(items) == 0 {
		return ui.FilePickerResult{}, fmt.Errorf("nenhum arquivo DOCX disponível no índice do vault")
	}
	result, err := ui.RunFilePicker("🍬 Selecionar DOCX para juntar", items, 2)
	if err != nil {
		return ui.FilePickerResult{}, err
	}
	if result.Action == ui.FilePickerConfirmed && len(result.Selected) < 2 {
		return ui.FilePickerResult{}, fmt.Errorf("selecione pelo menos dois arquivos DOCX")
	}
	return result, nil
}
