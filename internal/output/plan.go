package output

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Shape descreve se uma execução publica um arquivo ou um conjunto inseparável.
type Shape string

const (
	ShapeSingle Shape = "single"
	ShapeBundle Shape = "bundle"
)

var outputCategories = map[string]bool{
	"docx": true, "imagens": true, "impressao": true, "pdf": true, "rotinas": true,
}

// PlanRequest contém apenas decisões de domínio; raiz, data e colisões ficam centralizadas.
type PlanRequest struct {
	LibraryRoot string
	Category    string
	Shape       Shape
	BaseName    string
	Extension   string
	StartedAt   time.Time
}

// Plan mantém o destino público e a área descartável usada durante a produção.
type Plan struct {
	shape            Shape
	stageRoot        string
	workPath         string
	basePath         string
	finalPath        string
	dynamicExtension bool
	published        bool
}

// NewPlan prepara staging sem criar a árvore pública de resultados.
func NewPlan(request PlanRequest) (*Plan, error) {
	root := strings.TrimSpace(request.LibraryRoot)
	if root == "" {
		return nil, errors.New("a raiz da biblioteca não pode ficar vazia")
	}
	if !outputCategories[request.Category] {
		return nil, fmt.Errorf("categoria de output inválida: %s", request.Category)
	}
	if request.Shape != ShapeSingle && request.Shape != ShapeBundle {
		return nil, fmt.Errorf("formato de output inválido: %s", request.Shape)
	}
	base := SanitizeName(request.BaseName)
	if base == "" {
		base = "resultado"
	}
	extension := strings.TrimSpace(request.Extension)
	if request.Shape == ShapeSingle && extension != "" && !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	startedAt := request.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	categoryDir := filepath.Join(filepath.Clean(root), "resultados", startedAt.Format("2006-01-02"), request.Category)
	basePath := filepath.Join(categoryDir, base+extension)
	finalPath := uniqueOutputPath(basePath, request.Shape)
	stageRoot, err := os.MkdirTemp("", ".caramel-output-")
	if err != nil {
		return nil, fmt.Errorf("não foi possível criar staging de output: %w", err)
	}
	workName := filepath.Base(finalPath)
	return &Plan{
		shape: request.Shape, stageRoot: stageRoot,
		workPath: filepath.Join(stageRoot, workName), basePath: basePath, finalPath: finalPath,
		dynamicExtension: request.Shape == ShapeSingle && extension == "",
	}, nil
}

// SanitizeName cria um componente de caminho portável sem alterar acentos ou espaços úteis.
func SanitizeName(value string) string {
	value = strings.TrimSpace(filepath.Base(value))
	value = strings.Trim(value, ". ")
	replacer := strings.NewReplacer("<", "-", ">", "-", ":", "-", `"`, "-", "/", "-", `\`, "-", "|", "-", "?", "-", "*", "-")
	value = replacer.Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "- ")
}

// WorkPath é o arquivo ou diretório que a ferramenta deve produzir.
func (p *Plan) WorkPath() string {
	if p == nil {
		return ""
	}
	return p.workPath
}

// StageDir atende ferramentas que recebem um diretório mas publicam um único arquivo.
func (p *Plan) StageDir() string {
	if p == nil {
		return ""
	}
	return p.stageRoot
}

// FinalPath é o destino principal visível ao usuário.
func (p *Plan) FinalPath() string {
	if p == nil {
		return ""
	}
	return p.finalPath
}

// Publish move somente artefatos válidos para a árvore pública e traduz seus caminhos.
func (p *Plan) Publish(produced []string) ([]string, error) {
	if p == nil {
		return produced, nil
	}
	source := p.workPath
	if p.shape == ShapeSingle && len(produced) > 0 {
		for _, candidate := range produced {
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
				source = candidate
				break
			}
		}
	}
	info, err := os.Stat(source)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("a execução não produziu artefatos para publicar")
		}
		return nil, err
	}
	if p.shape == ShapeSingle && !info.Mode().IsRegular() {
		return nil, errors.New("o output único não é um arquivo regular")
	}
	if p.shape == ShapeBundle {
		if !info.IsDir() {
			return nil, errors.New("o pacote de output não é uma pasta")
		}
		empty, err := directoryEmpty(source)
		if err != nil {
			return nil, err
		}
		if empty {
			return nil, errors.New("a execução não produziu artefatos para publicar")
		}
	}
	if p.dynamicExtension {
		p.basePath += strings.ToLower(filepath.Ext(source))
	}

	if err := os.MkdirAll(filepath.Dir(p.finalPath), 0o755); err != nil {
		return nil, fmt.Errorf("não foi possível criar a pasta de resultados: %w", err)
	}
	reservedPath, release, err := reserveOutputPath(p.basePath, p.shape)
	if err != nil {
		return nil, err
	}
	defer release()
	p.finalPath = reservedPath
	if err := os.Rename(source, p.finalPath); err != nil {
		if err := copyThenPublish(source, p.finalPath, info); err != nil {
			return nil, err
		}
	}
	p.published = true

	translated := make([]string, 0, len(produced)+1)
	if len(produced) == 0 {
		return []string{p.finalPath}, nil
	}
	for _, path := range produced {
		if p.shape == ShapeSingle {
			translated = append(translated, p.finalPath)
			break
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			if filepath.Clean(path) == filepath.Clean(source) {
				translated = append(translated, p.finalPath)
			}
			continue
		}
		translated = append(translated, filepath.Join(p.finalPath, relative))
	}
	if len(translated) == 0 {
		translated = append(translated, p.finalPath)
	}
	return deduplicatePaths(translated), nil
}

// Translate converte um caminho de staging depois da publicação.
func (p *Plan) Translate(path string) string {
	if p == nil || !p.published || strings.TrimSpace(path) == "" {
		return path
	}
	if p.shape == ShapeSingle {
		return p.finalPath
	}
	relative, err := filepath.Rel(p.workPath, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return path
	}
	return filepath.Join(p.finalPath, relative)
}

// Cleanup remove somente o staging ainda não publicado.
func (p *Plan) Cleanup() {
	if p != nil && p.stageRoot != "" {
		_ = os.RemoveAll(p.stageRoot)
	}
}

func uniqueOutputPath(path string, shape Shape) string {
	for suffix := 1; ; suffix++ {
		candidate := outputPathWithSuffix(path, shape, suffix)
		if _, err := os.Lstat(candidate); err != nil {
			return candidate
		}
	}
}

func reserveOutputPath(path string, shape Shape) (string, func(), error) {
	for suffix := 1; ; suffix++ {
		candidate := outputPathWithSuffix(path, shape, suffix)
		lock := filepath.Join(filepath.Dir(candidate), ".caramel-reserve-"+filepath.Base(candidate))
		if err := os.Mkdir(lock, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", nil, fmt.Errorf("não foi possível reservar destino de output: %w", err)
		}
		release := func() { _ = os.Remove(lock) }
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, release, nil
		} else if err != nil {
			release()
			return "", nil, err
		}
		release()
	}
}

func outputPathWithSuffix(path string, shape Shape, suffix int) string {
	if suffix <= 1 {
		return path
	}
	extension := ""
	stem := path
	if shape == ShapeSingle {
		extension = filepath.Ext(path)
		stem = strings.TrimSuffix(path, extension)
	}
	return fmt.Sprintf("%s-%d%s", stem, suffix, extension)
}

func directoryEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	return len(entries) == 0, err
}

func copyThenPublish(source, destination string, info fs.FileInfo) error {
	parent := filepath.Dir(destination)
	temp, err := os.MkdirTemp(parent, ".caramel-publish-")
	if err != nil {
		return fmt.Errorf("não foi possível preparar publicação: %w", err)
	}
	defer os.RemoveAll(temp)
	staged := filepath.Join(temp, filepath.Base(destination))
	if info.IsDir() {
		err = copyDirectory(source, staged)
	} else {
		err = copyFile(source, staged, info.Mode())
	}
	if err != nil {
		return err
	}
	if err := os.Rename(staged, destination); err != nil {
		return fmt.Errorf("não foi possível publicar '%s': %w", destination, err)
	}
	_ = os.RemoveAll(source)
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(source, destination string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	outputFile, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(outputFile, input); err != nil {
		_ = outputFile.Close()
		return err
	}
	return outputFile.Close()
}

func deduplicatePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}
