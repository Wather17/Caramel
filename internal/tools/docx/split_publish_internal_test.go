package docx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishDOCXFilesRollsBackEarlierOutputsOnFailure(t *testing.T) {
	dir := t.TempDir()
	stagedFirst := filepath.Join(dir, "staged-first.docx")
	if err := os.WriteFile(stagedFirst, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	stagedMissing := filepath.Join(dir, "does-not-exist.docx")
	outputFirst := filepath.Join(dir, "output-first.docx")
	outputSecond := filepath.Join(dir, "output-second.docx")

	if err := publishDOCXFiles([]string{stagedFirst, stagedMissing}, []string{outputFirst, outputSecond}); err == nil {
		t.Fatal("publicação deveria falhar quando a segunda parte preparada não existe")
	}
	if _, err := os.Lstat(outputFirst); !os.IsNotExist(err) {
		t.Fatalf("primeira saída deveria ter sido revertida: stat err=%v", err)
	}
	if _, err := os.Lstat(outputSecond); !os.IsNotExist(err) {
		t.Fatalf("segunda saída não deveria ter sido criada: stat err=%v", err)
	}
}
