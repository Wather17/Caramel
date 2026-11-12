package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/output"
	"caramel/internal/vault"
)

func TestLibraryOutputIsIndexedImmediatelyAfterCommand(t *testing.T) {
	configureDOCXMergeTest(t)
	defer configureDOCXMergeTest(t)
	library := t.TempDir()
	t.Setenv("CARAMEL_LIBRARY_DIR", library)
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	store, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InitializeLibrary(context.Background(), library); err != nil {
		t.Fatal(err)
	}
	store.Close()
	dir := t.TempDir()
	first := filepath.Join(dir, "capa.docx")
	second := filepath.Join(dir, "atividade.docx")
	writeCLIForMergeDOCX(t, first, "capa")
	writeCLIForMergeDOCX(t, second, "atividade")
	args := []string{first, second}
	docxMergeCmd.SetContext(context.Background())
	startCLIExecution(docxMergeCmd, args)
	var stdout bytes.Buffer
	docxMergeCmd.SetOut(&stdout)
	if err := docxMergeCmd.RunE(docxMergeCmd, args); err != nil {
		t.Fatal(err)
	}
	store, err = vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	files, err := store.ListIndexedFiles(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range files {
		if strings.HasSuffix(file.Path, "capa_merged.docx") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("output não entrou no índice imediatamente: %#v", files)
	}
}

func TestCLIUsageRecorderPersistsStatusDurationAndCanonicalCommand(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", t.TempDir())
	versionCmd.SetContext(context.Background())
	beginCLIUsage(versionCmd)
	observeCLIUsage(versionCmd, output.StateWarning)
	finishCLIUsage(versionCmd, nil)

	store, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	report, err := store.DetailedCLIUsage(context.Background(), nil, vault.UsageCatalog{Commands: []string{"caramel version"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 1 || len(report.Commands) != 1 {
		t.Fatalf("uso canônico não foi registrado: %+v", report)
	}
	command := report.Commands[0]
	if command.Name != "caramel version" || command.Status.Warning != 1 || command.DurationSamples != 1 {
		t.Fatalf("status/duração inesperados: %+v", command)
	}
}

func TestCLIUsageRecorderIgnoresUsageCommand(t *testing.T) {
	usageCmd.SetContext(context.Background())
	beginCLIUsage(usageCmd)
	finishCLIUsage(usageCmd, nil)
	if recorder := cliUsageRecorderFrom(usageCmd); recorder != nil {
		t.Fatal("usage não deveria receber recorder de uso")
	}
}

func TestNormalizeCLIUsageStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		state  output.State
		err    error
		wanted string
	}{
		{name: "success", state: output.StateSuccess, wanted: vault.UsageStatusSuccess},
		{name: "warning", state: output.StateWarning, wanted: vault.UsageStatusWarning},
		{name: "skipped", state: output.StateSkipped, wanted: vault.UsageStatusWarning},
		{name: "canceled", state: output.StateCanceled, wanted: vault.UsageStatusCanceled},
		{name: "failed result", state: output.StateFailed, wanted: vault.UsageStatusFailed},
		{name: "error", state: output.StateSuccess, err: context.Canceled, wanted: vault.UsageStatusFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeCLIUsageStatus(test.state, test.err); got != test.wanted {
				t.Fatalf("status normalizado = %q, esperado %q", got, test.wanted)
			}
		})
	}
}
