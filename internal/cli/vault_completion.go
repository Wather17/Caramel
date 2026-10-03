package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

const vaultInputAnnotation = "caramel.io/indexed-inputs"

var (
	docxExtensions       = []string{".docx"}
	pdfExtensions        = []string{".pdf"}
	textExtensions       = []string{".txt"}
	colorizeExtensions   = []string{".png", ".jpg", ".jpeg", ".webp", ".docx"}
	printImageExtensions = []string{".png", ".jpg", ".jpeg", ".jpe", ".jfif", ".jif", ".webp", ".gif", ".bmp", ".tif", ".tiff"}
)

func init() {
	registerIndexedInputs(docxExtractCmd, docxExtensions)
	registerIndexedInputs(docxImagesListCmd, docxExtensions)
	registerIndexedInputs(docxImagesExtractCmd, docxExtensions)
	registerIndexedInputs(docxSplitCmd, docxExtensions)
	registerIndexedInputs(docxMergeCmd, docxExtensions)
	registerIndexedInputs(imageColorizeCmd, colorizeExtensions)
	registerIndexedInputs(cardsCmd, printImageExtensions)
	registerIndexedInputs(pdf2UpCmd, printImageExtensions)
	registerIndexedInputs(pdfCreateCmd, printImageExtensions)
	registerIndexedInputs(pdfMergeCmd, pdfExtensions)
	registerIndexedInputs(pdfSplitCmd, pdfExtensions)
	registerIndexedInputs(pdfPagesRenderCmd, pdfExtensions)
	registerIndexedInputs(pdfImagesExtractCmd, pdfExtensions)
	registerIndexedInputs(routineProcessCmd, docxExtensions)
	registerIndexedInputs(routineConsolidateCmd, docxExtensions)
	_ = imageGenerateCmd.RegisterFlagCompletionFunc("file", completeVaultFiles(textExtensions...))
	if imageGenerateCmd.Annotations == nil {
		imageGenerateCmd.Annotations = make(map[string]string)
	}
	imageGenerateCmd.Annotations[vaultInputAnnotation] = "flag:file"
}

func registerIndexedInputs(cmd *cobra.Command, extensions []string) {
	cmd.ValidArgsFunction = completeVaultFiles(extensions...)
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[vaultInputAnnotation] = "args"
}

func completeVaultFiles(extensions ...string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		store, err := vault.Open()
		if err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
		defer store.Close()
		if _, _, err := store.SyncIfStale(ctx, 30*time.Second); err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
		candidates, err := store.SearchIndexedFiles(ctx, vault.IndexedFileQuery{
			Text:       completionSearchText(toComplete),
			Extensions: extensions,
			Exclude:    args,
			Limit:      50,
		})
		if err != nil || len(candidates) == 0 {
			return nil, cobra.ShellCompDirectiveDefault
		}
		result := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			description := fmt.Sprintf("%s · %s", sourceRoleLabel(candidate.SourceRole), candidate.SourcePath)
			result = append(result, candidate.Path+"\t"+description)
		}
		return result, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder
	}
}

func completionSearchText(value string) string {
	value = strings.Trim(value, "\"'")
	return filepath.Base(value)
}

func sourceRoleLabel(role vault.SourceRole) string {
	switch role {
	case vault.SourceMaterials:
		return "materiais"
	case vault.SourceResults:
		return "resultados"
	default:
		return "fonte externa"
	}
}

func touchIndexedInputs(cmd *cobra.Command, args []string) {
	mode := cmd.Annotations[vaultInputAnnotation]
	if mode == "" {
		return
	}
	paths := args
	if strings.HasPrefix(mode, "flag:") {
		value, err := cmd.Flags().GetString(strings.TrimPrefix(mode, "flag:"))
		if err != nil || strings.TrimSpace(value) == "" {
			return
		}
		paths = []string{value}
	}
	store, err := vault.Open()
	if err != nil {
		return
	}
	defer store.Close()
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	for _, path := range paths {
		_ = store.TouchIndexedFile(ctx, path)
	}
}
