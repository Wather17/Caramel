package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/output"
	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

type openerCall struct {
	action string
	path   string
	bundle bool
}

type fakeResultOpener struct {
	calls []openerCall
	err   error
}

func (o *fakeResultOpener) Open(path string, bundle bool) error {
	o.calls = append(o.calls, openerCall{action: "open", path: path, bundle: bundle})
	return o.err
}

func (o *fakeResultOpener) Reveal(path string, bundle bool) error {
	o.calls = append(o.calls, openerCall{action: "reveal", path: path, bundle: bundle})
	return o.err
}

func useFakeResultOpener(t *testing.T, fake *fakeResultOpener) {
	t.Helper()
	previous := resultOpener
	resultOpener = fake
	t.Cleanup(func() { resultOpener = previous })
}

func recordCLIPathResult(t *testing.T, path string, shape vault.PathResultShape) string {
	t.Helper()
	store, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	run, err := store.StartPathRun(context.Background(), "teste", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPathRunResult(context.Background(), run.ID, "completed", nil, []string{path}, path, shape, nil); err != nil {
		t.Fatal(err)
	}
	return run.ID
}

func TestOpenAndRevealLastUseRecordedResultShape(t *testing.T) {
	tests := []struct {
		name   string
		reveal bool
		bundle bool
	}{
		{name: "open single", bundle: false},
		{name: "open bundle", bundle: true},
		{name: "reveal single", reveal: true, bundle: false},
		{name: "reveal bundle", reveal: true, bundle: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jsonFlag, quietFlag = false, false
			t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
			path := filepath.Join(t.TempDir(), "ação com espaço.pdf")
			shape := vault.PathResultSingle
			if test.bundle {
				shape = vault.PathResultBundle
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("resultado"), 0o600); err != nil {
				t.Fatal(err)
			}
			recordCLIPathResult(t, path, shape)
			fake := &fakeResultOpener{}
			useFakeResultOpener(t, fake)
			command := openLastCmd
			if test.reveal {
				command = revealLastCmd
			}
			var stdout bytes.Buffer
			command.SetOut(&stdout)
			if err := command.RunE(command, nil); err != nil {
				t.Fatal(err)
			}
			if len(fake.calls) != 1 || fake.calls[0].bundle != test.bundle || fake.calls[0].path != path {
				t.Fatalf("chamada inesperada: %#v", fake.calls)
			}
			wantAction := "open"
			if test.reveal {
				wantAction = "reveal"
			}
			if fake.calls[0].action != wantAction || !strings.Contains(stdout.String(), path) {
				t.Fatalf("resultado inesperado: call=%#v stdout=%q", fake.calls[0], stdout.String())
			}
		})
	}
}

func TestOpenLastReportsMissingAndRemovedResults(t *testing.T) {
	jsonFlag, quietFlag = false, false
	t.Run("vault vazio", func(t *testing.T) {
		t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
		fake := &fakeResultOpener{}
		useFakeResultOpener(t, fake)
		err := openLastCmd.RunE(openLastCmd, nil)
		if err == nil || !strings.Contains(err.Error(), "nenhum resultado") || len(fake.calls) != 0 {
			t.Fatalf("erro inesperado: %v calls=%#v", err, fake.calls)
		}
	})
	t.Run("caminho removido", func(t *testing.T) {
		t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
		path := filepath.Join(t.TempDir(), "removido.pdf")
		if err := os.WriteFile(path, []byte("resultado"), 0o600); err != nil {
			t.Fatal(err)
		}
		runID := recordCLIPathResult(t, path, vault.PathResultSingle)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		fake := &fakeResultOpener{}
		useFakeResultOpener(t, fake)
		err := openLastCmd.RunE(openLastCmd, nil)
		if err == nil || !strings.Contains(err.Error(), runID) || !strings.Contains(err.Error(), "caramel vault sync") || len(fake.calls) != 0 {
			t.Fatalf("erro inesperado: %v calls=%#v", err, fake.calls)
		}
	})
}

func TestOpenFlagRunsAfterPersistenceAndFailureDoesNotChangeHistory(t *testing.T) {
	jsonFlag, quietFlag = false, false
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	fake := &fakeResultOpener{err: errors.New("Explorer indisponível")}
	useFakeResultOpener(t, fake)
	command := &cobra.Command{Use: "producer", Annotations: map[string]string{domainExecutionAnnotation: "true"}}
	registerOpenFlag(command)
	if err := command.Flags().Set("open", "true"); err != nil {
		t.Fatal(err)
	}
	startCLIExecution(command, nil)
	path := filepath.Join(t.TempDir(), "resultado.pdf")
	if err := os.WriteFile(path, []byte("resultado"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	renderer, err := output.New(outputOptionsFor(command), &bytes.Buffer{}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Result(output.Result{Status: output.StateSuccess, Outputs: []string{path}}); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 || !strings.Contains(stderr.String(), "Explorer indisponível") {
		t.Fatalf("falha do opener não virou aviso: calls=%#v stderr=%q", fake.calls, stderr.String())
	}
	store, err := vault.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	last, err := store.LastPathResult(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if last.Status != "completed" || last.PrimaryPath != path {
		t.Fatalf("falha externa alterou o histórico: %+v", last)
	}
}

func TestOpenFlagDoesNotRunWhenPersistenceFails(t *testing.T) {
	jsonFlag, quietFlag = false, false
	fake := &fakeResultOpener{}
	useFakeResultOpener(t, fake)
	command := &cobra.Command{Use: "producer"}
	registerOpenFlag(command)
	if err := command.Flags().Set("open", "true"); err != nil {
		t.Fatal(err)
	}
	recorder := &cliExecutionRecorder{startErr: errors.New("vault indisponível")}
	command.SetContext(context.WithValue(context.Background(), executionContextKey{}, recorder))
	path := filepath.Join(t.TempDir(), "resultado.pdf")
	if err := os.WriteFile(path, []byte("resultado"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	renderer, err := output.New(outputOptionsFor(command), &bytes.Buffer{}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Result(output.Result{Status: output.StateSuccess, Outputs: []string{path}}); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 0 || !strings.Contains(stderr.String(), "vault indisponível") {
		t.Fatalf("abertura deveria aguardar persistência: calls=%#v stderr=%q", fake.calls, stderr.String())
	}
}

func TestOpenFlagIsOptInAndRejectsAutomationModes(t *testing.T) {
	fake := &fakeResultOpener{}
	useFakeResultOpener(t, fake)
	command := &cobra.Command{Use: "producer"}
	registerOpenFlag(command)
	path := filepath.Join(t.TempDir(), "resultado.pdf")
	if err := os.WriteFile(path, []byte("resultado"), 0o600); err != nil {
		t.Fatal(err)
	}
	renderer, err := output.New(outputOptionsFor(command), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Result(output.Result{Status: output.StateSuccess, Outputs: []string{path}}); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("opener foi chamado sem --open: %#v", fake.calls)
	}

	if err := command.Flags().Set("open", "true"); err != nil {
		t.Fatal(err)
	}
	previousJSON, previousQuiet := jsonFlag, quietFlag
	t.Cleanup(func() { jsonFlag, quietFlag = previousJSON, previousQuiet })
	for _, mode := range []struct {
		name  string
		json  bool
		quiet bool
	}{{name: "json", json: true}, {name: "quiet", quiet: true}} {
		t.Run(mode.name, func(t *testing.T) {
			jsonFlag, quietFlag = mode.json, mode.quiet
			err := RootCmd.PersistentPreRunE(command, nil)
			if err == nil || !strings.Contains(err.Error(), "--open") {
				t.Fatalf("combinação deveria ser rejeitada: %v", err)
			}
			if executionRecorderFrom(command) != nil {
				t.Fatal("rejeição deveria acontecer antes de iniciar a run")
			}
		})
	}
}

func TestOpenFlagIsRegisteredOnlyOnProducingCommands(t *testing.T) {
	producers := []*cobra.Command{
		docxExtractCmd, docxImagesExtractCmd, docxSplitCmd, docxMergeCmd,
		imageColorizeCmd, imageGenerateCmd, cardsCmd, pdf2UpCmd,
		pdfCreateCmd, pdfMergeCmd, pdfSplitCmd, pdfPagesRenderCmd, pdfImagesExtractCmd,
		routineProcessCmd, routineConsolidateCmd,
	}
	for _, command := range producers {
		if command.Flags().Lookup("open") == nil {
			t.Errorf("%s deveria aceitar --open", command.CommandPath())
		}
	}
	if docxImagesListCmd.Flags().Lookup("open") != nil {
		t.Fatal("comando somente de leitura não deveria aceitar --open")
	}
}
