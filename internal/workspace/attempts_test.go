package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caramel/internal/tools/ai"
)

func TestWorkspacePersisteTentativasRedigidasELeManifestoAntigo(t *testing.T) {
	t.Setenv("CARAMEL_WORKSPACE_DIR", t.TempDir())
	project, err := CreateProject("Metadados")
	if err != nil {
		t.Fatal(err)
	}
	run, err := project.BeginRun("generate", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt := ai.AttemptMetadata{Operation: "image_generation", RequestedModel: "modelo", ItemName: "bolo", Status: "success", CorrelationID: "caramel-safe"}
	if err := project.FinishRunWithAttempts(run.ID, "completed", nil, nil, []ai.AttemptMetadata{attempt}); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenProject(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Runs) != 1 || len(reopened.Runs[0].Attempts) != 1 || reopened.Runs[0].Attempts[0].SchemaVersion != ai.AttemptSchemaVersion {
		t.Fatalf("tentativa não persistida: %+v", reopened.Runs)
	}
	data, err := json.Marshal(reopened.Runs[0])
	if err != nil || strings.Contains(string(data), "Authorization") || strings.Contains(string(data), "prompt completo") {
		t.Fatalf("manifesto expôs dado sensível: %s err=%v", data, err)
	}

	legacy := `{"id":"legacy","name":"Antigo","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","runs":[{"id":"r1","operation":"generate","status":"completed","started_at":"2026-01-01T00:00:00Z"}]}`
	legacyDir := filepath.Join(filepath.Dir(project.Directory), "legacy")
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, manifestName), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	old, err := OpenProject("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Runs) != 1 || len(old.Runs[0].Attempts) != 0 {
		t.Fatalf("manifesto antigo deveria continuar legível: %+v", old.Runs)
	}
}
