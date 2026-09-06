package vault

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"caramel/internal/workspace"
)

// MigrateLegacyProjects converte os manifestos JSON do MVP em coleções do vault.
// A operação é idempotente e não remove os diretórios antigos.
func (v *Vault) MigrateLegacyProjects(ctx context.Context) (MigrationReport, error) {
	projects, err := workspace.ListProjects()
	if err != nil {
		return MigrationReport{}, err
	}
	var report MigrationReport
	for _, project := range projects {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		var existing string
		err := v.db.QueryRowContext(ctx, "SELECT collection_id FROM legacy_migrations WHERE legacy_project_id=?", project.ID).Scan(&existing)
		if err == nil || err != sql.ErrNoRows {
			continue
		}

		collection, err := v.legacyCollection(ctx, project.ID, project.Name)
		if err != nil {
			return report, err
		}
		mapping := make(map[string]string, len(project.Assets))
		for _, asset := range project.Assets {
			path := project.Resolve(asset.Path)
			if _, statErr := os.Stat(path); statErr != nil {
				continue
			}
			result, importErr := v.ImportFile(ctx, path, asset.Name, []string{"legacy", "importado"})
			if importErr != nil {
				return report, fmt.Errorf("falha ao migrar material '%s': %w", asset.Name, importErr)
			}
			mapping[asset.ID] = result.Material.ID
			if result.Created {
				report.Materials++
			}
			if err := v.AddToCollection(ctx, collection.ID, result.Material.ID); err != nil {
				return report, err
			}
		}

		for _, legacyRun := range project.Runs {
			var migratedRun string
			runErr := v.db.QueryRowContext(ctx, "SELECT run_id FROM legacy_runs WHERE legacy_project_id=? AND legacy_run_id=?", project.ID, legacyRun.ID).Scan(&migratedRun)
			if runErr == nil || runErr != sql.ErrNoRows {
				continue
			}
			inputs := mapLegacyIDs(legacyRun.Inputs, mapping)
			run, startErr := v.StartRun(ctx, collection.ID, legacyRun.Operation, legacyRun.Options, inputs)
			if startErr != nil {
				return report, startErr
			}
			if _, err := v.db.ExecContext(ctx, "INSERT INTO legacy_runs(legacy_project_id,legacy_run_id,run_id) VALUES (?,?,?)", project.ID, legacyRun.ID, run.ID); err != nil {
				return report, err
			}
			var outputs []string
			for _, artifact := range legacyRun.Artifacts {
				path := project.Resolve(artifact.Path)
				if _, statErr := os.Stat(path); statErr != nil {
					continue
				}
				result, importErr := v.ImportFile(ctx, path, artifact.Name, []string{"legacy", artifact.Step})
				if importErr != nil {
					return report, importErr
				}
				outputs = append(outputs, result.Material.ID)
				if result.Created {
					report.Materials++
				}
				if err := v.AddToCollection(ctx, collection.ID, result.Material.ID); err != nil {
					return report, err
				}
			}
			for _, output := range outputs {
				for _, input := range inputs {
					if err := v.AddDerivation(ctx, run.ID, []string{input}, []string{output}); err != nil {
						return report, err
					}
				}
			}
			status := legacyRun.Status
			if status == "" {
				status = "completed"
			}
			if err := v.FinishRun(ctx, run.ID, status, outputs, errorFromText(legacyRun.Error)); err != nil {
				return report, err
			}
			report.Runs++
		}

		if _, err := v.db.ExecContext(ctx, "INSERT INTO legacy_migrations(legacy_project_id,collection_id,migrated_at) VALUES (?,?,?)", project.ID, collection.ID, nowString()); err != nil {
			return report, err
		}
		report.Projects++
	}
	return report, nil
}

func (v *Vault) legacyCollection(ctx context.Context, legacyID, name string) (Collection, error) {
	var collection Collection
	var temporary int
	var created, updated string
	err := v.db.QueryRowContext(ctx, "SELECT id,name,temporary,created_at,updated_at FROM collections WHERE legacy_id=?", legacyID).Scan(&collection.ID, &collection.Name, &temporary, &created, &updated)
	if err == nil {
		collection.Temporary = temporary != 0
		collection.CreatedAt, collection.UpdatedAt = parseTime(created), parseTime(updated)
		return collection, nil
	}
	if err != sql.ErrNoRows {
		return Collection{}, err
	}
	return v.CreateCollectionWithLegacy(ctx, name, true, legacyID)
}

// CreateCollectionWithLegacy é usado apenas pela migração de manifestos antigos.
func (v *Vault) CreateCollectionWithLegacy(ctx context.Context, name string, temporary bool, legacyID string) (Collection, error) {
	id := fmt.Sprintf("collection-%d", time.Now().UnixNano())
	now := nowString()
	if _, err := v.db.ExecContext(ctx, "INSERT INTO collections(id,name,temporary,legacy_id,created_at,updated_at) VALUES (?,?,?,?,?,?)", id, name, boolInt(temporary), legacyID, now, now); err != nil {
		return Collection{}, err
	}
	return Collection{ID: id, Name: name, Temporary: temporary, CreatedAt: parseTime(now), UpdatedAt: parseTime(now)}, nil
}

func mapLegacyIDs(ids []string, mapping map[string]string) []string {
	var result []string
	for _, id := range ids {
		if mapped := mapping[id]; mapped != "" {
			result = append(result, mapped)
		}
	}
	return result
}

func errorFromText(message string) error {
	if message == "" {
		return nil
	}
	return fmt.Errorf("%s", message)
}
