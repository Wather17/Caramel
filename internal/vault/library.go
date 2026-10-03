package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SourceRole descreve como uma pasta participa da biblioteca visível.
type SourceRole string

const (
	SourceMaterials SourceRole = "materials"
	SourceResults   SourceRole = "results"
	SourceExternal  SourceRole = "external"
)

// IndexedSource é uma raiz vinculada ao índice, sem transferência de posse.
type IndexedSource struct {
	ID            int64      `json:"id"`
	Path          string     `json:"path"`
	Role          SourceRole `json:"role"`
	LastScannedAt *time.Time `json:"last_scanned_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// IndexedFile guarda somente localização e metadados de um arquivo vinculado.
type IndexedFile struct {
	ID           int64      `json:"id"`
	SourceID     int64      `json:"source_id"`
	Path         string     `json:"path"`
	RelativePath string     `json:"relative_path"`
	Name         string     `json:"name"`
	Extension    string     `json:"extension"`
	Size         int64      `json:"size"`
	ModifiedAt   time.Time  `json:"modified_at"`
	ContentHash  string     `json:"content_hash,omitempty"`
	FirstSeenAt  time.Time  `json:"first_seen_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	Available    bool       `json:"available"`
}

// SyncReport resume uma atualização incremental de uma ou mais fontes.
type SyncReport struct {
	Sources     int      `json:"sources"`
	Files       int      `json:"files"`
	Created     int      `json:"created"`
	Updated     int      `json:"updated"`
	Moved       int      `json:"moved"`
	Unavailable int      `json:"unavailable"`
	Unchanged   int      `json:"unchanged"`
	Warnings    []string `json:"warnings,omitempty"`
}

// IndexStatus resume o estado persistido sem varrer o sistema de arquivos.
type IndexStatus struct {
	Sources     int `json:"sources"`
	Available   int `json:"available"`
	Unavailable int `json:"unavailable"`
}

type scannedFile struct {
	path         string
	relativePath string
	name         string
	extension    string
	size         int64
	modifiedAt   time.Time
	hash         string
}

// InitializeLibrary cria a parte visível e registra materiais e resultados.
func (v *Vault) InitializeLibrary(ctx context.Context, root string) ([]IndexedSource, error) {
	root, err := cleanAbsolutePath(root)
	if err != nil {
		return nil, err
	}
	materials := filepath.Join(root, "materiais")
	results := filepath.Join(root, "resultados")
	for _, path := range []string{materials, results} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("falha ao criar '%s': %w", path, err)
		}
	}
	first, err := v.AddSource(ctx, materials, SourceMaterials)
	if err != nil {
		return nil, err
	}
	second, err := v.AddSource(ctx, results, SourceResults)
	if err != nil {
		return nil, err
	}
	return []IndexedSource{first, second}, nil
}

// AddSource registra uma pasta existente sem copiar ou modificar seu conteúdo.
func (v *Vault) AddSource(ctx context.Context, path string, role SourceRole) (IndexedSource, error) {
	if v == nil || v.db == nil {
		return IndexedSource{}, errors.New("vault não está aberto")
	}
	if role != SourceMaterials && role != SourceResults && role != SourceExternal {
		return IndexedSource{}, fmt.Errorf("papel de fonte inválido: %s", role)
	}
	path, err := cleanAbsolutePath(path)
	if err != nil {
		return IndexedSource{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return IndexedSource{}, fmt.Errorf("não foi possível acessar a fonte '%s': %w", path, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return IndexedSource{}, fmt.Errorf("a fonte '%s' precisa ser uma pasta real", path)
	}
	now := nowString()
	_, err = v.db.ExecContext(ctx, `INSERT INTO indexed_sources(path,role,created_at,updated_at)
		VALUES(?,?,?,?) ON CONFLICT(path) DO UPDATE SET role=excluded.role,updated_at=excluded.updated_at`, path, string(role), now, now)
	if err != nil {
		return IndexedSource{}, fmt.Errorf("falha ao registrar fonte: %w", err)
	}
	return v.sourceByPath(ctx, path)
}

// ListSources retorna fontes em ordem estável de papel e caminho.
func (v *Vault) ListSources(ctx context.Context) ([]IndexedSource, error) {
	rows, err := v.db.QueryContext(ctx, `SELECT id,path,role,last_scanned_at,created_at,updated_at
		FROM indexed_sources ORDER BY CASE role WHEN 'materials' THEN 0 WHEN 'results' THEN 1 ELSE 2 END,path COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []IndexedSource
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, source)
	}
	return result, rows.Err()
}

// Sync atualiza todas as fontes e continua quando uma delas está inacessível.
func (v *Vault) Sync(ctx context.Context, full bool) (SyncReport, error) {
	sources, err := v.ListSources(ctx)
	if err != nil {
		return SyncReport{}, err
	}
	var total SyncReport
	for _, source := range sources {
		report, syncErr := v.syncSource(ctx, source, full)
		total.Sources++
		total.Files += report.Files
		total.Created += report.Created
		total.Updated += report.Updated
		total.Moved += report.Moved
		total.Unavailable += report.Unavailable
		total.Unchanged += report.Unchanged
		total.Warnings = append(total.Warnings, report.Warnings...)
		if syncErr != nil {
			total.Warnings = append(total.Warnings, fmt.Sprintf("%s: %v", source.Path, syncErr))
		}
	}
	return total, nil
}

// SyncIfStale sincroniza somente quando alguma fonte excede maxAge.
func (v *Vault) SyncIfStale(ctx context.Context, maxAge time.Duration) (SyncReport, bool, error) {
	sources, err := v.ListSources(ctx)
	if err != nil {
		return SyncReport{}, false, err
	}
	now := time.Now()
	for _, source := range sources {
		if source.LastScannedAt == nil || now.Sub(*source.LastScannedAt) > maxAge {
			report, err := v.Sync(ctx, false)
			return report, true, err
		}
	}
	return SyncReport{}, false, nil
}

// Status retorna contagens persistidas sem tocar nas fontes.
func (v *Vault) Status(ctx context.Context) (IndexStatus, error) {
	var status IndexStatus
	if err := v.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM indexed_sources").Scan(&status.Sources); err != nil {
		return status, err
	}
	if err := v.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM indexed_files WHERE available=1").Scan(&status.Available); err != nil {
		return status, err
	}
	if err := v.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM indexed_files WHERE available=0").Scan(&status.Unavailable); err != nil {
		return status, err
	}
	return status, nil
}

// ListIndexedFiles retorna o índice para diagnóstico e completadores futuros.
func (v *Vault) ListIndexedFiles(ctx context.Context, includeUnavailable bool) ([]IndexedFile, error) {
	query := indexedFileSelect
	if !includeUnavailable {
		query += " WHERE available=1"
	}
	query += " ORDER BY path COLLATE NOCASE"
	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []IndexedFile
	for rows.Next() {
		item, err := scanIndexedFile(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// TouchIndexedFile registra uso sem alterar o arquivo fonte.
func (v *Vault) TouchIndexedFile(ctx context.Context, path string) error {
	path, err := cleanAbsolutePath(path)
	if err != nil {
		return err
	}
	_, err = v.db.ExecContext(ctx, "UPDATE indexed_files SET last_used_at=? WHERE path=?", nowString(), path)
	return err
}

func (v *Vault) syncSource(ctx context.Context, source IndexedSource, full bool) (SyncReport, error) {
	existing, err := v.filesBySource(ctx, source.ID)
	if err != nil {
		return SyncReport{}, err
	}
	scanned, complete, warnings, err := scanSourceFiles(ctx, source.Path, existing, full)
	if err != nil {
		return SyncReport{Warnings: warnings}, err
	}
	report := SyncReport{Files: len(scanned), Warnings: warnings}
	seen := make(map[string]bool, len(scanned))
	for _, item := range scanned {
		seen[item.path] = true
	}

	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	scanTime := nowString()
	usedMoveIDs := make(map[int64]bool)
	for _, item := range scanned {
		old, exists := existing[item.path]
		if exists {
			changed := old.Size != item.size || !old.ModifiedAt.Equal(item.modifiedAt) || (full && old.ContentHash != item.hash)
			if item.hash == "" {
				item.hash = old.ContentHash
			}
			_, err = tx.ExecContext(ctx, `UPDATE indexed_files SET relative_path=?,name=?,extension=?,size=?,modified_at=?,content_hash=?,last_seen_at=?,available=1 WHERE id=?`,
				item.relativePath, item.name, item.extension, item.size, timeString(item.modifiedAt), item.hash, scanTime, old.ID)
			if err != nil {
				return report, err
			}
			if changed {
				report.Updated++
			} else {
				report.Unchanged++
			}
			continue
		}

		move := findMoveCandidate(existing, seen, usedMoveIDs, item.hash)
		if move == nil {
			move, err = findGlobalMoveCandidate(ctx, tx, seen, usedMoveIDs, item.hash)
			if err != nil {
				return report, err
			}
		}
		if move != nil {
			_, err = tx.ExecContext(ctx, `UPDATE indexed_files SET source_id=?,path=?,relative_path=?,name=?,extension=?,size=?,modified_at=?,content_hash=?,last_seen_at=?,available=1 WHERE id=?`,
				source.ID, item.path, item.relativePath, item.name, item.extension, item.size, timeString(item.modifiedAt), item.hash, scanTime, move.ID)
			if err != nil {
				return report, err
			}
			usedMoveIDs[move.ID] = true
			report.Moved++
			continue
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO indexed_files(source_id,path,relative_path,name,extension,size,modified_at,content_hash,first_seen_at,last_seen_at,available)
			VALUES(?,?,?,?,?,?,?,?,?,?,1)`, source.ID, item.path, item.relativePath, item.name, item.extension, item.size, timeString(item.modifiedAt), item.hash, scanTime, scanTime)
		if err != nil {
			return report, err
		}
		report.Created++
	}

	if complete {
		for path, old := range existing {
			if seen[path] || usedMoveIDs[old.ID] || !old.Available {
				continue
			}
			if _, err := tx.ExecContext(ctx, "UPDATE indexed_files SET available=0,last_seen_at=? WHERE id=?", scanTime, old.ID); err != nil {
				return report, err
			}
			report.Unavailable++
		}
		if _, err := tx.ExecContext(ctx, "UPDATE indexed_sources SET last_scanned_at=?,updated_at=? WHERE id=?", scanTime, scanTime, source.ID); err != nil {
			return report, err
		}
	}
	if err := tx.Commit(); err != nil {
		return report, err
	}
	return report, nil
}

func scanSourceFiles(ctx context.Context, root string, existing map[string]IndexedFile, full bool) ([]scannedFile, bool, []string, error) {
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("não é uma pasta")
		}
		return nil, false, nil, err
	}
	var result []scannedFile
	var warnings []string
	complete := true
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if walkErr != nil {
			complete = false
			warnings = append(warnings, fmt.Sprintf("não foi possível acessar %s: %v", path, walkErr))
			return nil
		}
		if path != root && shouldIgnoreIndexedEntry(path, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			if err != nil {
				complete = false
				warnings = append(warnings, fmt.Sprintf("não foi possível ler %s: %v", path, err))
			}
			return nil
		}
		absolute, err := cleanAbsolutePath(path)
		if err != nil {
			complete = false
			warnings = append(warnings, err.Error())
			return nil
		}
		relative, err := filepath.Rel(root, absolute)
		if err != nil {
			return err
		}
		item := scannedFile{path: absolute, relativePath: relative, name: info.Name(), extension: strings.ToLower(filepath.Ext(info.Name())), size: info.Size(), modifiedAt: info.ModTime().UTC()}
		old, exists := existing[absolute]
		if full || !exists || old.Size != item.size || !old.ModifiedAt.Equal(item.modifiedAt) || old.ContentHash == "" {
			hash, size, hashErr := hashFile(absolute)
			if hashErr != nil {
				complete = false
				warnings = append(warnings, hashErr.Error())
				return nil
			}
			item.hash, item.size = hash, size
		} else {
			item.hash = old.ContentHash
		}
		result = append(result, item)
		return nil
	})
	if err != nil {
		return nil, false, warnings, err
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result, complete, warnings, nil
}

func shouldIgnoreIndexedEntry(path string, entry os.DirEntry) bool {
	name := entry.Name()
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~$") || strings.HasPrefix(name, ".caramel-") || isPlatformHidden(path)
}

func findMoveCandidate(existing map[string]IndexedFile, seen map[string]bool, used map[int64]bool, hash string) *IndexedFile {
	if hash == "" {
		return nil
	}
	paths := make([]string, 0, len(existing))
	for path := range existing {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		item := existing[path]
		if seen[path] || used[item.ID] || item.ContentHash != hash {
			continue
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			candidate := item
			return &candidate
		}
	}
	return nil
}

func findGlobalMoveCandidate(ctx context.Context, tx *sql.Tx, seen map[string]bool, used map[int64]bool, hash string) (*IndexedFile, error) {
	if hash == "" {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx, indexedFileSelect+" WHERE content_hash=? ORDER BY path COLLATE NOCASE", hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []IndexedFile
	for rows.Next() {
		item, err := scanIndexedFile(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, item := range candidates {
		if seen[item.Path] || used[item.ID] {
			continue
		}
		if _, err := os.Stat(item.Path); errors.Is(err, os.ErrNotExist) {
			candidate := item
			return &candidate, nil
		}
	}
	return nil, nil
}

func (v *Vault) filesBySource(ctx context.Context, sourceID int64) (map[string]IndexedFile, error) {
	rows, err := v.db.QueryContext(ctx, indexedFileSelect+" WHERE source_id=?", sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]IndexedFile)
	for rows.Next() {
		item, err := scanIndexedFile(rows)
		if err != nil {
			return nil, err
		}
		result[item.Path] = item
	}
	return result, rows.Err()
}

func (v *Vault) sourceByPath(ctx context.Context, path string) (IndexedSource, error) {
	row := v.db.QueryRowContext(ctx, `SELECT id,path,role,last_scanned_at,created_at,updated_at FROM indexed_sources WHERE path=?`, path)
	return scanSource(row)
}

type sourceScanner interface{ Scan(...any) error }

func scanSource(row sourceScanner) (IndexedSource, error) {
	var source IndexedSource
	var role, created, updated string
	var scanned sql.NullString
	if err := row.Scan(&source.ID, &source.Path, &role, &scanned, &created, &updated); err != nil {
		return source, err
	}
	source.Role = SourceRole(role)
	source.CreatedAt, source.UpdatedAt = parseTime(created), parseTime(updated)
	if scanned.Valid {
		t := parseTime(scanned.String)
		source.LastScannedAt = &t
	}
	return source, nil
}

const indexedFileSelect = `SELECT id,source_id,path,relative_path,name,extension,size,modified_at,content_hash,first_seen_at,last_seen_at,last_used_at,available FROM indexed_files`

func scanIndexedFile(row sourceScanner) (IndexedFile, error) {
	var item IndexedFile
	var modified, firstSeen, lastSeen string
	var lastUsed sql.NullString
	var available int
	if err := row.Scan(&item.ID, &item.SourceID, &item.Path, &item.RelativePath, &item.Name, &item.Extension, &item.Size, &modified, &item.ContentHash, &firstSeen, &lastSeen, &lastUsed, &available); err != nil {
		return item, err
	}
	item.ModifiedAt, item.FirstSeenAt, item.LastSeenAt = parseTime(modified), parseTime(firstSeen), parseTime(lastSeen)
	item.Available = available != 0
	if lastUsed.Valid {
		t := parseTime(lastUsed.String)
		item.LastUsedAt = &t
	}
	return item, nil
}

func cleanAbsolutePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("o caminho não pode ficar vazio")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("caminho inválido '%s': %w", path, err)
	}
	return filepath.Clean(absolute), nil
}
