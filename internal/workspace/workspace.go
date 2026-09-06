// Package workspace gerencia projetos persistentes usados pela TUI do Caramel.
package workspace

import (
	"crypto/sha256"
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
	"unicode"
)

const (
	manifestName = "project.json"
	workDir      = "work"
	inputsDir    = "inputs"
	outputsDir   = "outputs"
	runsDir      = "runs"
	logsDir      = "logs"
)

// Asset representa um arquivo importado para o projeto.
type Asset struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Kind       string    `json:"kind"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"`
	ImportedAt time.Time `json:"imported_at"`
}

// ImageFile é uma referência uniforme a uma imagem importada ou gerada.
type ImageFile struct {
	ID   string
	Name string
	Path string
}

// Artifact representa um arquivo produzido por uma etapa do projeto.
type Artifact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	Step      string    `json:"step"`
	CreatedAt time.Time `json:"created_at"`
}

// Run registra uma execução, inclusive execuções parcialmente concluídas.
type Run struct {
	ID         string            `json:"id"`
	Operation  string            `json:"operation"`
	Status     string            `json:"status"`
	Inputs     []string          `json:"inputs,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
	Artifacts  []Artifact        `json:"artifacts,omitempty"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// Project é o manifesto persistente de um projeto do Caramel.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Assets    []Asset   `json:"assets,omitempty"`
	Runs      []Run     `json:"runs,omitempty"`

	// Directory é derivado do local do manifesto e não é serializado.
	Directory string `json:"-"`
}

// RootDir retorna a raiz global dos projetos do usuário.
func RootDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("CARAMEL_WORKSPACE_DIR")); override != "" {
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
		return filepath.Join(base, "caramel", "workspaces"), nil
	}

	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "caramel", "workspaces"), nil
}

// CreateProject cria um projeto autocontido e nomeado.
func CreateProject(name string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("o nome do projeto não pode ficar vazio")
	}

	root, err := RootDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, fmt.Errorf("falha ao criar a raiz do workspace: %w", err)
	}

	baseID := slugify(name)
	if baseID == "" {
		baseID = "projeto"
	}
	id := baseID
	for i := 2; ; i++ {
		candidate := filepath.Join(root, id)
		_, statErr := os.Stat(candidate)
		if errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if statErr != nil {
			return nil, fmt.Errorf("falha ao verificar projeto existente: %w", statErr)
		}
		id = fmt.Sprintf("%s-%d", baseID, i)
	}

	now := time.Now().UTC()
	p := &Project{ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Directory: filepath.Join(root, id)}
	for _, dir := range []string{inputsDir, workDir, outputsDir, runsDir, logsDir} {
		if err := os.MkdirAll(filepath.Join(p.Directory, dir), 0700); err != nil {
			return nil, fmt.Errorf("falha ao criar estrutura do projeto: %w", err)
		}
	}
	if err := p.Save(); err != nil {
		return nil, err
	}
	return p, nil
}

// ListProjects lista projetos válidos em ordem de atualização mais recente.
func ListProjects() ([]*Project, error) {
	root, err := RootDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao listar workspaces: %w", err)
	}

	projects := make([]*Project, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p, err := OpenProject(entry.Name())
		if err != nil {
			// Pastas incompletas não impedem a TUI de abrir os demais projetos.
			continue
		}
		projects = append(projects, p)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].UpdatedAt.After(projects[j].UpdatedAt)
	})
	return projects, nil
}

// OpenProject abre um projeto pelo identificador seguro do diretório.
func OpenProject(id string) (*Project, error) {
	root, err := RootDir()
	if err != nil {
		return nil, err
	}
	if filepath.Base(id) != id || id == "." || id == ".." {
		return nil, errors.New("identificador de projeto inválido")
	}
	path := filepath.Join(root, id, manifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir projeto '%s': %w", id, err)
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("manifesto do projeto '%s' inválido: %w", id, err)
	}
	p.Directory = filepath.Dir(path)
	return &p, nil
}

// Save persiste o manifesto do projeto usando escrita temporária e substituição atômica.
func (p *Project) Save() error {
	if p == nil || p.Directory == "" {
		return errors.New("projeto sem diretório de persistência")
	}
	p.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar projeto: %w", err)
	}
	tmp, err := os.CreateTemp(p.Directory, ".project-*.tmp")
	if err != nil {
		return fmt.Errorf("falha ao criar manifesto temporário: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("falha ao proteger manifesto temporário: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("falha ao escrever manifesto: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("falha ao fechar manifesto: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(p.Directory, manifestName)); err != nil {
		return fmt.Errorf("falha ao publicar manifesto: %w", err)
	}
	return nil
}

// ImportPaths copia imagens de arquivos ou do primeiro nível de diretórios para inputs/.
func (p *Project) ImportPaths(paths []string) ([]Asset, error) {
	if p == nil || p.Directory == "" {
		return nil, errors.New("projeto inválido")
	}
	var files []string
	for _, raw := range paths {
		path := filepath.Clean(strings.TrimSpace(raw))
		if path == "." || path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("não foi possível acessar '%s': %w", path, err)
		}
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, fmt.Errorf("falha ao ler '%s': %w", path, err)
			}
			for _, entry := range entries {
				if !entry.IsDir() && isImage(entry.Name()) {
					files = append(files, filepath.Join(path, entry.Name()))
				}
			}
		} else if isImage(path) {
			files = append(files, path)
		} else {
			return nil, fmt.Errorf("'%s' não é uma imagem PNG, JPG, JPEG ou WEBP", path)
		}
	}
	sort.Strings(files)

	var imported []Asset
	for _, file := range files {
		asset, added, err := p.importFile(file)
		if err != nil {
			return imported, err
		}
		if added {
			imported = append(imported, asset)
		}
	}
	if len(imported) > 0 {
		if err := p.Save(); err != nil {
			return imported, err
		}
	}
	return imported, nil
}

func (p *Project) importFile(source string) (Asset, bool, error) {
	hash, size, err := fileHash(source)
	if err != nil {
		return Asset{}, false, err
	}
	for _, asset := range p.Assets {
		if asset.SHA256 == hash {
			return asset, false, nil
		}
	}

	name := filepath.Base(source)
	destination := filepath.Join(p.Directory, inputsDir, name)
	if _, err := os.Stat(destination); err == nil {
		stem, ext := strings.TrimSuffix(name, filepath.Ext(name)), filepath.Ext(name)
		for i := 2; ; i++ {
			candidate := filepath.Join(p.Directory, inputsDir, fmt.Sprintf("%s-%d%s", stem, i, ext))
			if _, candidateErr := os.Stat(candidate); errors.Is(candidateErr, os.ErrNotExist) {
				destination = candidate
				name = filepath.Base(candidate)
				break
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Asset{}, false, fmt.Errorf("falha ao verificar destino de '%s': %w", name, err)
	}

	if err := copyFile(source, destination); err != nil {
		return Asset{}, false, err
	}
	asset := Asset{
		ID:         hash[:16],
		Name:       name,
		Path:       filepath.ToSlash(filepath.Join(inputsDir, name)),
		Kind:       "image",
		Size:       size,
		SHA256:     hash,
		ImportedAt: time.Now().UTC(),
	}
	p.Assets = append(p.Assets, asset)
	return asset, true, nil
}

// ImageAssets retorna os ativos de imagem importados, em ordem de nome.
func (p *Project) ImageAssets() []Asset {
	assets := make([]Asset, 0, len(p.Assets))
	for _, asset := range p.Assets {
		if asset.Kind == "image" {
			assets = append(assets, asset)
		}
	}
	sort.Slice(assets, func(i, j int) bool { return strings.ToLower(assets[i].Name) < strings.ToLower(assets[j].Name) })
	return assets
}

// ImageFiles combina imagens importadas e imagens produzidas por execuções
// anteriores, permitindo encadear etapas dentro da TUI.
func (p *Project) ImageFiles() []ImageFile {
	files := make([]ImageFile, 0, len(p.Assets))
	for _, asset := range p.Assets {
		if asset.Kind == "image" {
			files = append(files, ImageFile{ID: asset.ID, Name: asset.Name, Path: asset.Path})
		}
	}
	for _, run := range p.Runs {
		for _, artifact := range run.Artifacts {
			if artifact.Kind == "image" {
				files = append(files, ImageFile{ID: artifact.ID, Name: artifact.Name, Path: artifact.Path})
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name) })
	return files
}

// Resolve transforma um caminho relativo do projeto em caminho absoluto.
func (p *Project) Resolve(relative string) string {
	return filepath.Join(p.Directory, filepath.FromSlash(relative))
}

// OutputDir cria o diretório previsível de uma etapa e execução.
func (p *Project) OutputDir(step, runID string) string {
	dir := filepath.Join(p.Directory, outputsDir, step, runID)
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// WorkDir cria o diretório de intermediários de uma execução.
func (p *Project) WorkDir(runID string) string {
	dir := filepath.Join(p.Directory, workDir, runID)
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// BeginRun adiciona uma execução pendente ao histórico.
func (p *Project) BeginRun(operation string, inputs []string, options map[string]string) (*Run, error) {
	runID := fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102-150405"), randomSuffix())
	run := Run{ID: runID, Operation: operation, Status: "running", Inputs: append([]string(nil), inputs...), Options: options, StartedAt: time.Now().UTC()}
	p.Runs = append(p.Runs, run)
	if err := p.Save(); err != nil {
		return nil, err
	}
	return &p.Runs[len(p.Runs)-1], nil
}

// FinishRun fecha uma execução e publica seus artefatos no manifesto.
func (p *Project) FinishRun(runID, status string, artifacts []Artifact, runErr error) error {
	for i := range p.Runs {
		if p.Runs[i].ID != runID {
			continue
		}
		now := time.Now().UTC()
		p.Runs[i].Status = status
		p.Runs[i].FinishedAt = &now
		p.Runs[i].Artifacts = append([]Artifact(nil), artifacts...)
		if runErr != nil {
			p.Runs[i].Error = runErr.Error()
		}
		return p.Save()
	}
	return fmt.Errorf("execução '%s' não encontrada", runID)
}

func fileHash(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("falha ao abrir '%s': %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("falha ao calcular hash de '%s': %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("falha ao abrir origem '%s': %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("falha ao criar cópia '%s': %w", destination, err)
	}
	if _, copyErr := io.Copy(out, in); copyErr != nil {
		_ = out.Close()
		_ = os.Remove(destination)
		return fmt.Errorf("falha ao copiar arquivo: %w", copyErr)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("falha ao fechar cópia: %w", err)
	}
	return nil
}

func isImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	default:
		return false
	}
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func randomSuffix() string {
	return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
}
