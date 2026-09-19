package vault

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"caramel/internal/tools/ai"
)

func TestVaultPersisteTentativasComSchemaVersionadoERedaction(t *testing.T) {
	t.Setenv("CARAMEL_VAULT_DIR", t.TempDir())
	v, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	run, err := v.StartRun(context.Background(), "", "generate", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	attempts := []ai.AttemptMetadata{
		{Operation: "text_analysis", RequestedModel: "modelo", Attempt: 1, Status: "failure", ErrorClass: "transient", StatusCode: 502},
		{Operation: "text_analysis", RequestedModel: "modelo", Attempt: 2, Status: "success", Usage: ai.UsageMetadata{TotalTokens: 12}},
	}
	if err := v.FinishRunWithAttempts(context.Background(), run.ID, "completed", nil, nil, attempts); err != nil {
		t.Fatal(err)
	}
	got, err := v.RunAttempts(context.Background(), run.ID)
	if err != nil || len(got) != 2 || got[0].Sequence != 1 || got[1].Usage.TotalTokens != 12 {
		t.Fatalf("tentativas inesperadas: %+v err=%v", got, err)
	}
	data, err := json.Marshal(got)
	if err != nil || strings.Contains(string(data), "sk-") || strings.Contains(string(data), "prompt") {
		t.Fatalf("metadados não redigidos: %s err=%v", data, err)
	}
	var version int
	if err := v.db.QueryRow("SELECT MAX(version) FROM schema_versions").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("schema deveria registrar versão %d, obteve %d", schemaVersion, version)
	}
}
