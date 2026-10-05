package cli

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"caramel/internal/ui"

	"github.com/spf13/cobra"
)

func TestDOCXMergeCommandUsesOutputFlagAndReportsResult(t *testing.T) {
	configureDOCXMergeTest(t)
	defer configureDOCXMergeTest(t)
	dir := t.TempDir()
	first := filepath.Join(dir, "first.docx")
	second := filepath.Join(dir, "second.docx")
	output := filepath.Join(dir, "merged.docx")
	writeCLIForMergeDOCX(t, first, "um")
	writeCLIForMergeDOCX(t, second, "dois")
	if err := docxMergeCmd.Flags().Set("output", output); err != nil {
		t.Fatalf("não foi possível configurar --output: %v", err)
	}

	oldOut := docxMergeCmd.OutOrStdout()
	var stdout bytes.Buffer
	docxMergeCmd.SetOut(&stdout)
	defer docxMergeCmd.SetOut(oldOut)
	if err := docxMergeCmd.RunE(docxMergeCmd, []string{first, second}); err != nil {
		t.Fatalf("docx merge falhou: %v", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("o DOCX final não foi criado: %v", err)
	}
	if !strings.Contains(stdout.String(), "DOCX juntados: 2 arquivo(s)") || !strings.Contains(stdout.String(), output) {
		t.Errorf("a saída da CLI não informa resumo e destino: %q", stdout.String())
	}
}

func TestDOCXMergeCommandRequiresTwoInputsAndOutput(t *testing.T) {
	configureDOCXMergeTest(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CARAMEL_LIBRARY_DIR", "")
	if err := docxMergeCmd.Args(docxMergeCmd, []string{"one.docx"}); err == nil {
		t.Fatal("docx merge deveria exigir dois argumentos")
	}
	if err := docxMergeCmd.RunE(docxMergeCmd, []string{"one.docx", "two.docx"}); err == nil || !strings.Contains(err.Error(), "--output") {
		t.Fatalf("docx merge deveria exigir --output/-o sem biblioteca, erro=%v", err)
	}
}

func TestDOCXMergeArgsAllowPickerOnlyInInteractiveHumanMode(t *testing.T) {
	configureDOCXMergeTest(t)
	previousInteractive := interactiveInput
	interactiveInput = func(io.Reader) bool { return true }
	t.Cleanup(func() { interactiveInput = previousInteractive })
	command := &cobra.Command{}
	command.SetIn(strings.NewReader(""))
	if err := validateDOCXMergeArgs(command, nil); err != nil {
		t.Fatalf("argumentos vazios deveriam abrir o seletor: %v", err)
	}

	jsonFlag = true
	if err := validateDOCXMergeArgs(command, nil); err == nil {
		t.Fatalf("modo JSON deveria exigir argumentos explícitos: %v", err)
	}
}

func TestDOCXMergePickRejectsNonInteractiveAndExplicitFiles(t *testing.T) {
	configureDOCXMergeTest(t)
	docxMergePick = true
	command := &cobra.Command{}
	command.SetIn(strings.NewReader(""))
	if err := validateDOCXMergeArgs(command, []string{"one.docx", "two.docx"}); err == nil || !strings.Contains(err.Error(), "não pode") {
		t.Fatalf("--pick deveria rejeitar arquivos explícitos: %v", err)
	}
	if err := validateDOCXMergeArgs(command, nil); err == nil || !strings.Contains(err.Error(), "terminal interativo") {
		t.Fatalf("--pick deveria rejeitar entrada não interativa: %v", err)
	}
}

func TestDOCXMergePickerRevealDoesNotMerge(t *testing.T) {
	configureDOCXMergeTest(t)
	previousInteractive := interactiveInput
	interactiveInput = func(io.Reader) bool { return true }
	t.Cleanup(func() { interactiveInput = previousInteractive })
	docxMergePick = true
	docxMergePicker = func(*cobra.Command) (ui.FilePickerResult, error) {
		return ui.FilePickerResult{Action: ui.FilePickerReveal, RevealPath: `C:\docs\atividade.docx`}, nil
	}
	fake := &fakeResultOpener{}
	previousOpener := docxMergeOpener
	docxMergeOpener = fake
	t.Cleanup(func() { docxMergeOpener = previousOpener })
	var stdout bytes.Buffer
	docxMergeCmd.SetOut(&stdout)
	if err := docxMergeCmd.RunE(docxMergeCmd, nil); err != nil {
		t.Fatalf("revelar pelo picker falhou: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0].action != "reveal" || fake.calls[0].path != `C:\docs\atividade.docx` || fake.calls[0].bundle {
		t.Fatalf("chamada de reveal inesperada: %#v", fake.calls)
	}
	if !strings.Contains(stdout.String(), "atividade.docx") {
		t.Fatalf("saída deveria informar o arquivo revelado: %q", stdout.String())
	}
}

func TestDOCXMergeCommandUsesLibraryDefault(t *testing.T) {
	configureDOCXMergeTest(t)
	defer configureDOCXMergeTest(t)
	library := t.TempDir()
	t.Setenv("CARAMEL_LIBRARY_DIR", library)
	t.Setenv("CARAMEL_VAULT_DIR", filepath.Join(t.TempDir(), "vault"))
	dir := t.TempDir()
	first := filepath.Join(dir, "first.docx")
	second := filepath.Join(dir, "second.docx")
	writeCLIForMergeDOCX(t, first, "um")
	writeCLIForMergeDOCX(t, second, "dois")
	var stdout bytes.Buffer
	docxMergeCmd.SetOut(&stdout)
	if err := docxMergeCmd.RunE(docxMergeCmd, []string{first, second}); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("resultados", time.Now().Format("2006-01-02"), "docx", "first_merged.docx")
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("destino diário não informado: %q", stdout.String())
	}
}

func configureDOCXMergeTest(t *testing.T) {
	t.Helper()
	docxMergeOutput = ""
	docxMergePick = false
	docxMergePicker = pickDOCXMergeFiles
	docxMergeCmd.Flags().Lookup("output").Changed = false
	docxMergeCmd.Flags().Lookup("pick").Changed = false
	verboseFlag = false
	quietFlag = false
	jsonFlag = false
}

func writeCLIForMergeDOCX(t *testing.T, destination, text string) {
	t.Helper()
	file, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entries := map[string]string{
		"[Content_Types].xml": fmt.Sprintf(`<Types xmlns="%s"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`, "http://schemas.openxmlformats.org/package/2006/content-types"),
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   fmt.Sprintf(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>%s</w:t></w:r></w:p><w:sectPr/></w:body></w:document>`, text),
	}
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
