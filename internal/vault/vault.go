// Package vault implementa o acervo global de materiais do Caramel.
package vault

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

// Material é a unidade atômica do acervo global.
type Material struct {
	ID          string
	Title       string
	Description string
	Kind        string
	Category    string
	Extension   string
	ContentHash string
	Version     int
	Size        int64
	ObjectPath  string
	SourceName  string
	SourcePath  string
	Tags        []string
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Collection é uma visão temporária sobre materiais do vault.
type Collection struct {
	ID        string
	Name      string
	Temporary bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Run registra uma operação feita sobre materiais.
type Run struct {
	ID           string
	CollectionID string
	Operation    string
	Status       string
	Options      map[string]string
	Inputs       []string
	Outputs      []string
	Error        string
	StartedAt    time.Time
	FinishedAt   *time.Time
}

// SearchOptions define a busca na inbox/biblioteca.
type SearchOptions struct {
	Query           string
	Kind            string
	Category        string
	Tag             string
	IncludeArchived bool
}

// ImportResult informa se um arquivo novo foi criado ou se já existia.
type ImportResult struct {
	Material Material
	Created  bool
}

// MigrationReport resume a conversão de projetos antigos.
type MigrationReport struct {
	Projects  int
	Materials int
	Runs      int
}

// Vault mantém a conexão SQLite e o diretório de objetos imutáveis.
type Vault struct {
	db      *sql.DB
	root    string
	objects string
}

// RootDir retorna o diretório de dados do vault.
func RootDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("CARAMEL_VAULT_DIR")); override != "" {
		return filepath.Clean(override), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("não foi possível obter o diretório pessoal: %w", err)
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "caramel"), nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "caramel"), nil
}

// Open inicializa o vault, aplica o schema e habilita integridade referencial.
func Open() (*Vault, error) {
	root, err := RootDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "objects"), 0700); err != nil {
		return nil, fmt.Errorf("falha ao criar diretório do vault: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "vault.sqlite"))
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir vault SQLite: %w", err)
	}
	v := &Vault{db: db, root: root, objects: filepath.Join(root, "objects")}
	if err := v.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return v, nil
}

// Close fecha a conexão do vault.
func (v *Vault) Close() error {
	if v == nil || v.db == nil {
		return nil
	}
	return v.db.Close()
}

func (v *Vault) configure() error {
	for _, pragma := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000", "PRAGMA journal_mode = WAL"} {
		if _, err := v.db.Exec(pragma); err != nil {
			return fmt.Errorf("falha ao configurar SQLite: %w", err)
		}
	}
	if _, err := v.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("falha ao criar schema do vault: %w", err)
	}
	if err := v.ensureCategoryColumn(); err != nil {
		return err
	}
	if _, err := v.db.Exec("CREATE INDEX IF NOT EXISTS idx_materials_category ON materials(category)"); err != nil {
		return fmt.Errorf("falha ao criar índice de categorias: %w", err)
	}
	if _, err := v.db.Exec("INSERT OR IGNORE INTO schema_versions(version, applied_at) VALUES (?, ?)", schemaVersion, nowString()); err != nil {
		return fmt.Errorf("falha ao registrar versão do schema: %w", err)
	}
	return nil
}

func (v *Vault) ensureCategoryColumn() error {
	rows, err := v.db.Query("PRAGMA table_info(materials)")
	if err != nil {
		return fmt.Errorf("falha ao inspecionar schema de materiais: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("falha ao ler schema de materiais: %w", err)
		}
		if name == "category" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("falha ao finalizar leitura do schema: %w", err)
	}
	if _, err := v.db.Exec("ALTER TABLE materials ADD COLUMN category TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("falha ao migrar coluna de categoria: %w", err)
	}
	return nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS schema_versions (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS materials (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    extension TEXT NOT NULL,
    content_hash TEXT NOT NULL UNIQUE,
    version INTEGER NOT NULL DEFAULT 1,
    size INTEGER NOT NULL,
    object_path TEXT NOT NULL UNIQUE,
    source_name TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    archived_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_materials_title ON materials(title);
CREATE INDEX IF NOT EXISTS idx_materials_kind ON materials(kind);
CREATE INDEX IF NOT EXISTS idx_materials_archived ON materials(archived_at);
CREATE TABLE IF NOT EXISTS material_tags (
    material_id TEXT NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY(material_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_material_tags_tag ON material_tags(tag);
CREATE TABLE IF NOT EXISTS collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    temporary INTEGER NOT NULL DEFAULT 1,
    legacy_id TEXT UNIQUE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS collection_materials (
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    material_id TEXT NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    added_at TEXT NOT NULL,
    PRIMARY KEY(collection_id, material_id)
);
CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,
    collection_id TEXT REFERENCES collections(id) ON DELETE SET NULL,
    operation TEXT NOT NULL,
    status TEXT NOT NULL,
    options_json TEXT NOT NULL DEFAULT '{}',
    error TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT
);
CREATE TABLE IF NOT EXISTS run_inputs (
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    material_id TEXT NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    PRIMARY KEY(run_id, material_id)
);
CREATE TABLE IF NOT EXISTS run_outputs (
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    material_id TEXT NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    PRIMARY KEY(run_id, material_id)
);
CREATE TABLE IF NOT EXISTS material_derivations (
    parent_id TEXT NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    child_id TEXT NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    PRIMARY KEY(parent_id, child_id, run_id)
);
CREATE TABLE IF NOT EXISTS legacy_migrations (
    legacy_project_id TEXT PRIMARY KEY,
    collection_id TEXT NOT NULL REFERENCES collections(id),
    migrated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS legacy_runs (
    legacy_project_id TEXT NOT NULL,
    legacy_run_id TEXT NOT NULL,
    run_id TEXT NOT NULL REFERENCES runs(id),
    PRIMARY KEY(legacy_project_id, legacy_run_id)
);
`

// ImportFile copia um arquivo para o vault e deduplica por hash de conteúdo.
func (v *Vault) ImportFile(ctx context.Context, source, title string, tags []string) (ImportResult, error) {
	if v == nil || v.db == nil {
		return ImportResult{}, errors.New("vault não está aberto")
	}
	if err := ctx.Err(); err != nil {
		return ImportResult{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return ImportResult{}, fmt.Errorf("não foi possível acessar '%s': %w", source, err)
	}
	if !info.Mode().IsRegular() {
		return ImportResult{}, fmt.Errorf("'%s' não é um arquivo regular", source)
	}
	hash, size, err := hashFile(source)
	if err != nil {
		return ImportResult{}, err
	}
	metadata := inferFilenameMetadata(source)
	importTags := cleanTags(append(append([]string{}, tags...), metadata.Tags...))
	existing, err := v.getByHash(hash)
	if err != nil {
		return ImportResult{}, err
	}
	if existing != nil {
		if err := v.mergeImportedMetadata(ctx, existing, metadata.Category, importTags); err != nil {
			return ImportResult{}, err
		}
		return ImportResult{Material: *existing}, nil
	}

	ext := canonicalExtension(source)
	objectName := hash + ext
	temp, err := os.CreateTemp(v.objects, ".import-*")
	if err != nil {
		return ImportResult{}, fmt.Errorf("falha ao criar objeto temporário: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := copyInto(temp, source); err != nil {
		_ = temp.Close()
		return ImportResult{}, err
	}
	if err := temp.Close(); err != nil {
		return ImportResult{}, err
	}
	objectPath := filepath.Join(v.objects, objectName)
	if err := os.Rename(tempName, objectPath); err != nil && !errors.Is(err, os.ErrExist) {
		return ImportResult{}, fmt.Errorf("falha ao publicar objeto: %w", err)
	}

	if title == "" {
		title = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	}
	material := Material{
		ID: "sha256:" + hash, Title: title, Kind: kindForExtension(ext), Category: metadata.Category, Extension: ext,
		ContentHash: hash, Version: 1, Size: size, ObjectPath: objectName,
		SourceName: filepath.Base(source), SourcePath: source, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Tags: importTags,
	}
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO materials
        (id,title,description,kind,category,extension,content_hash,version,size,object_path,source_name,source_path,created_at,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, material.ID, material.Title, material.Description, material.Kind, material.Category, material.Extension,
		material.ContentHash, material.Version, material.Size, material.ObjectPath, material.SourceName, material.SourcePath, timeString(material.CreatedAt), timeString(material.UpdatedAt)); err != nil {
		_ = tx.Rollback()
		if existing, lookupErr := v.getByHash(hash); lookupErr == nil && existing != nil {
			_ = os.Remove(objectPath)
			return ImportResult{Material: *existing}, nil
		}
		return ImportResult{}, fmt.Errorf("falha ao registrar material: %w", err)
	}
	for _, tag := range material.Tags {
		if _, err := tx.ExecContext(ctx, "INSERT INTO material_tags(material_id, tag) VALUES (?, ?)", material.ID, tag); err != nil {
			_ = tx.Rollback()
			return ImportResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Material: material, Created: true}, nil
}

// ImportPaths importa arquivos ou o primeiro nível de uma pasta.
func (v *Vault) ImportPaths(ctx context.Context, paths []string, tags []string) ([]ImportResult, error) {
	var files []string
	for _, raw := range paths {
		path := filepath.Clean(strings.TrimSpace(raw))
		if path == "" || path == "." {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("não foi possível acessar '%s': %w", path, err)
		}
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if !entry.IsDir() && isSupportedImage(entry.Name()) {
					files = append(files, filepath.Join(path, entry.Name()))
				}
			}
		} else if isSupportedImage(path) {
			files = append(files, path)
		} else {
			return nil, fmt.Errorf("'%s' não é uma imagem suportada", path)
		}
	}
	sort.Strings(files)
	results := make([]ImportResult, 0, len(files))
	for _, file := range files {
		result, err := v.ImportFile(ctx, file, "", tags)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

// GetMaterial retorna um material ativo ou arquivado pelo ID.
func (v *Vault) GetMaterial(ctx context.Context, id string) (*Material, error) {
	row := v.db.QueryRowContext(ctx, materialSelect+" WHERE m.id = ?", id)
	return scanMaterial(row, v.loadTags(ctx, id))
}

// Search procura materiais por título, nome de origem, descrição, tag e tipo.
func (v *Vault) Search(ctx context.Context, opts SearchOptions) ([]Material, error) {
	query := `SELECT DISTINCT m.id FROM materials m LEFT JOIN material_tags t ON t.material_id = m.id WHERE 1=1`
	args := []any{}
	if !opts.IncludeArchived {
		query += " AND m.archived_at IS NULL"
	}
	if opts.Query != "" {
		query += " AND (LOWER(m.title) LIKE LOWER(?) OR LOWER(m.description) LIKE LOWER(?) OR LOWER(m.source_name) LIKE LOWER(?) OR LOWER(m.category) LIKE LOWER(?) OR LOWER(t.tag) LIKE LOWER(?))"
		term := "%" + opts.Query + "%"
		normalizedTerm := "%" + normalizeFilenameText(opts.Query) + "%"
		args = append(args, term, term, term, normalizedTerm, normalizedTerm)
	}
	if opts.Kind != "" {
		query += " AND m.kind = ?"
		args = append(args, opts.Kind)
	}
	if opts.Category != "" {
		query += " AND m.category = ?"
		args = append(args, opts.Category)
	}
	if opts.Tag != "" {
		query += " AND t.tag = ?"
		args = append(args, opts.Tag)
	}
	query += " ORDER BY m.updated_at DESC, m.title COLLATE NOCASE"
	rows, err := v.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var materials []Material
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		material, err := v.GetMaterial(ctx, id)
		if err != nil {
			return nil, err
		}
		materials = append(materials, *material)
	}
	return materials, rows.Err()
}

func (v *Vault) mergeImportedMetadata(ctx context.Context, material *Material, category string, tags []string) error {
	mergedTags := cleanTags(append(append([]string{}, material.Tags...), tags...))
	mergedCategory := material.Category
	if mergedCategory == "" {
		mergedCategory = category
	}
	shouldUnarchive := material.ArchivedAt != nil
	if mergedCategory == material.Category && strings.Join(mergedTags, "\x00") == strings.Join(material.Tags, "\x00") && !shouldUnarchive {
		return nil
	}

	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	updated := nowString()
	if _, err := tx.ExecContext(ctx, "UPDATE materials SET category=?, archived_at=NULL, updated_at=? WHERE id=?", mergedCategory, updated, material.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM material_tags WHERE material_id=?", material.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, tag := range mergedTags {
		if _, err := tx.ExecContext(ctx, "INSERT INTO material_tags(material_id,tag) VALUES (?,?)", material.ID, tag); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	material.Category = mergedCategory
	material.Tags = mergedTags
	material.UpdatedAt = parseTime(updated)
	material.ArchivedAt = nil
	return nil
}

// ObjectPath resolve o arquivo físico de um material.
func (v *Vault) ObjectPath(material Material) string {
	return filepath.Join(v.objects, material.ObjectPath)
}

// CreateCollection cria uma coleção de referências.
func (v *Vault) CreateCollection(ctx context.Context, name string, temporary bool, legacyID string) (Collection, error) {
	if strings.TrimSpace(name) == "" {
		return Collection{}, errors.New("o nome da coleção não pode ficar vazio")
	}
	id := fmt.Sprintf("collection-%d", time.Now().UnixNano())
	now := nowString()
	_, err := v.db.ExecContext(ctx, "INSERT INTO collections(id,name,temporary,legacy_id,created_at,updated_at) VALUES (?,?,?,?,?,?)", id, name, boolInt(temporary), nullableString(legacyID), now, now)
	if err != nil {
		return Collection{}, err
	}
	return Collection{ID: id, Name: name, Temporary: temporary, CreatedAt: parseTime(now), UpdatedAt: parseTime(now)}, nil
}

// AddToCollection adiciona uma referência sem duplicá-la.
func (v *Vault) AddToCollection(ctx context.Context, collectionID, materialID string) error {
	_, err := v.db.ExecContext(ctx, "INSERT OR IGNORE INTO collection_materials(collection_id,material_id,added_at) VALUES (?,?,?)", collectionID, materialID, nowString())
	return err
}

// CollectionMaterials retorna os materiais ativos de uma coleção.
func (v *Vault) CollectionMaterials(ctx context.Context, collectionID string) ([]Material, error) {
	rows, err := v.db.QueryContext(ctx, "SELECT m.id FROM materials m JOIN collection_materials cm ON cm.material_id=m.id WHERE cm.collection_id=? AND m.archived_at IS NULL ORDER BY cm.added_at", collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Material
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		material, err := v.GetMaterial(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, *material)
	}
	return result, rows.Err()
}

// StartRun cria uma execução no banco.
func (v *Vault) StartRun(ctx context.Context, collectionID, operation string, options map[string]string, inputs []string) (Run, error) {
	id := fmt.Sprintf("run-%d", time.Now().UnixNano())
	data, _ := json.Marshal(options)
	started := nowString()
	_, err := v.db.ExecContext(ctx, "INSERT INTO runs(id,collection_id,operation,status,options_json,started_at) VALUES (?,?,?,?,?,?)", id, nullableString(collectionID), operation, "running", string(data), started)
	if err != nil {
		return Run{}, err
	}
	if err := v.addRunMaterials(ctx, "run_inputs", id, inputs); err != nil {
		return Run{}, err
	}
	return Run{ID: id, CollectionID: collectionID, Operation: operation, Status: "running", Options: options, Inputs: inputs, StartedAt: parseTime(started)}, nil
}

// FinishRun fecha a execução e registra seus outputs.
func (v *Vault) FinishRun(ctx context.Context, runID, status string, outputs []string, runErr error) error {
	finished := nowString()
	message := ""
	if runErr != nil {
		message = runErr.Error()
	}
	if _, err := v.db.ExecContext(ctx, "UPDATE runs SET status=?, error=?, finished_at=? WHERE id=?", status, message, finished, runID); err != nil {
		return err
	}
	return v.addRunMaterials(ctx, "run_outputs", runID, outputs)
}

// AddDerivation registra a proveniência automática entre materiais.
func (v *Vault) AddDerivation(ctx context.Context, runID string, parents, children []string) error {
	for _, parent := range parents {
		for _, child := range children {
			if _, err := v.db.ExecContext(ctx, "INSERT OR IGNORE INTO material_derivations(parent_id,child_id,run_id) VALUES (?,?,?)", parent, child, runID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ArchiveMaterial remove o material das buscas ativas sem apagar o objeto.
func (v *Vault) ArchiveMaterial(ctx context.Context, id string) error {
	_, err := v.db.ExecContext(ctx, "UPDATE materials SET archived_at=?, updated_at=? WHERE id=?", nowString(), nowString(), id)
	return err
}

// UpdateMetadata altera título, descrição e tags sem alterar o objeto físico.
func (v *Vault) UpdateMetadata(ctx context.Context, id, title, description string, tags []string) error {
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE materials SET title=?, description=?, updated_at=? WHERE id=?", title, description, nowString(), id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM material_tags WHERE material_id=?", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, tag := range cleanTags(tags) {
		if _, err := tx.ExecContext(ctx, "INSERT INTO material_tags(material_id,tag) VALUES (?,?)", id, tag); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (v *Vault) addRunMaterials(ctx context.Context, table, runID string, ids []string) error {
	for _, id := range ids {
		if _, err := v.db.ExecContext(ctx, "INSERT OR IGNORE INTO "+table+"(run_id,material_id) VALUES (?,?)", runID, id); err != nil {
			return err
		}
	}
	return nil
}

const materialSelect = `SELECT m.id,m.title,m.description,m.kind,m.category,m.extension,m.content_hash,m.version,m.size,m.object_path,m.source_name,m.source_path,m.archived_at,m.created_at,m.updated_at FROM materials m`

type scanner interface{ Scan(...any) error }

func scanMaterial(row scanner, tags []string) (*Material, error) {
	var m Material
	var archived sql.NullString
	var created, updated string
	if err := row.Scan(&m.ID, &m.Title, &m.Description, &m.Kind, &m.Category, &m.Extension, &m.ContentHash, &m.Version, &m.Size, &m.ObjectPath, &m.SourceName, &m.SourcePath, &archived, &created, &updated); err != nil {
		return nil, err
	}
	if archived.Valid && archived.String != "" {
		t := parseTime(archived.String)
		m.ArchivedAt = &t
	}
	m.CreatedAt, m.UpdatedAt, m.Tags = parseTime(created), parseTime(updated), tags
	return &m, nil
}

func (v *Vault) getByHash(hash string) (*Material, error) {
	row := v.db.QueryRow(materialSelect+" WHERE m.content_hash = ?", hash)
	material, err := scanMaterial(row, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	material.Tags = v.loadTags(context.Background(), material.ID)
	return material, nil
}

func (v *Vault) loadTags(ctx context.Context, id string) []string {
	if id == "" {
		return nil
	}
	rows, err := v.db.QueryContext(ctx, "SELECT tag FROM material_tags WHERE material_id=? ORDER BY tag", id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if rows.Scan(&tag) == nil {
			tags = append(tags, tag)
		}
	}
	return tags
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("falha ao abrir '%s': %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func copyInto(dst *os.File, source string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err := io.Copy(dst, in); err != nil {
		return fmt.Errorf("falha ao copiar objeto: %w", err)
	}
	return nil
}

func canonicalExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".jpeg" {
		return ".jpg"
	}
	if ext == "" {
		return ".bin"
	}
	return ext
}

func kindForExtension(ext string) string {
	switch ext {
	case ".png", ".jpg", ".webp":
		return "image"
	default:
		return "file"
	}
}

func isSupportedImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	default:
		return false
	}
}

func cleanTags(tags []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, raw := range tags {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	sort.Strings(result)
	return result
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nowString() string                 { return time.Now().UTC().Format(time.RFC3339Nano) }
func timeString(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func parseTime(value string) time.Time  { t, _ := time.Parse(time.RFC3339Nano, value); return t }
