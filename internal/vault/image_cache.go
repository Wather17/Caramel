package vault

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var imageLibrarySlugPattern = regexp.MustCompile(`[^\p{L}\p{N}_-]+`)

// GeneratedImage is an image in the user's local generation library.
type GeneratedImage struct {
	Key          string
	Name         string
	Prompt       string
	Extension    string
	RelativePath string
	Path         string
}

// ImageLibraryDir returns the browsable image-library directory without creating it.
func (v *Vault) ImageLibraryDir() string {
	if v == nil {
		return ""
	}
	return filepath.Join(v.root, "image-library")
}

// LookupGeneratedImage finds the current image associated with a generation key.
func (v *Vault) LookupGeneratedImage(ctx context.Context, key string) (GeneratedImage, bool, error) {
	if v == nil || v.db == nil {
		return GeneratedImage{}, false, errors.New("vault não está aberto")
	}
	if err := ctx.Err(); err != nil {
		return GeneratedImage{}, false, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return GeneratedImage{}, false, errors.New("chave da biblioteca de imagens vazia")
	}

	var entry GeneratedImage
	err := v.db.QueryRowContext(ctx, `SELECT cache_key, name, prompt, extension, relative_path
		FROM generated_image_cache WHERE cache_key = ?`, key).Scan(
		&entry.Key, &entry.Name, &entry.Prompt, &entry.Extension, &entry.RelativePath,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return GeneratedImage{}, false, nil
	}
	if err != nil {
		return GeneratedImage{}, false, fmt.Errorf("falha ao consultar biblioteca de imagens: %w", err)
	}
	if !filepath.IsLocal(entry.RelativePath) || filepath.Base(entry.RelativePath) != entry.RelativePath {
		return GeneratedImage{}, false, errors.New("caminho inválido na biblioteca de imagens")
	}
	entry.Extension = strings.TrimPrefix(strings.ToLower(entry.Extension), ".")
	switch entry.Extension {
	case "png", "jpg", "webp":
	default:
		return GeneratedImage{}, false, errors.New("formato inválido na biblioteca de imagens")
	}
	if strings.ToLower(filepath.Ext(entry.RelativePath)) != "."+entry.Extension {
		return GeneratedImage{}, false, errors.New("extensão inconsistente na biblioteca de imagens")
	}
	libraryDir := v.ImageLibraryDir()
	dirInfo, err := os.Lstat(libraryDir)
	if err != nil {
		return GeneratedImage{}, false, fmt.Errorf("não foi possível acessar a biblioteca de imagens: %w", err)
	}
	if !dirInfo.IsDir() {
		return GeneratedImage{}, false, errors.New("diretório da biblioteca de imagens não é um diretório regular")
	}
	entry.Path = filepath.Join(libraryDir, entry.RelativePath)
	info, err := os.Lstat(entry.Path)
	if err != nil {
		return GeneratedImage{}, false, fmt.Errorf("não foi possível acessar a imagem da biblioteca: %w", err)
	}
	if !info.Mode().IsRegular() {
		return GeneratedImage{}, false, errors.New("arquivo da biblioteca de imagens não é regular")
	}
	return entry, true, nil
}

// StoreGeneratedImage saves an image with a readable filename and points the key to it.
// Previous files remain in the library when a key is refreshed.
func (v *Vault) StoreGeneratedImage(ctx context.Context, key, name, prompt, extension string, data []byte) (GeneratedImage, error) {
	if v == nil || v.db == nil {
		return GeneratedImage{}, errors.New("vault não está aberto")
	}
	if err := ctx.Err(); err != nil {
		return GeneratedImage{}, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return GeneratedImage{}, errors.New("chave da biblioteca de imagens vazia")
	}
	if len(data) == 0 {
		return GeneratedImage{}, errors.New("imagem da biblioteca vazia")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "imagem"
	}
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	if extension == "" {
		extension = "png"
	}
	switch extension {
	case "png", "jpg", "webp":
	default:
		return GeneratedImage{}, fmt.Errorf("formato de imagem não suportado pela biblioteca: %q", extension)
	}

	libraryDir := v.ImageLibraryDir()
	if err := os.MkdirAll(libraryDir, 0700); err != nil {
		return GeneratedImage{}, fmt.Errorf("falha ao criar biblioteca de imagens: %w", err)
	}
	dirInfo, err := os.Lstat(libraryDir)
	if err != nil {
		return GeneratedImage{}, fmt.Errorf("falha ao inspecionar biblioteca de imagens: %w", err)
	}
	if !dirInfo.IsDir() {
		return GeneratedImage{}, errors.New("caminho da biblioteca de imagens não é um diretório regular")
	}
	keyHash := sha256.Sum256([]byte(key))
	contentHash := sha256.Sum256(data)
	keyID := hex.EncodeToString(keyHash[:])[:12]
	contentID := hex.EncodeToString(contentHash[:])[:12]
	fileName := fmt.Sprintf("%s-%s-%s.%s", imageLibrarySlug(name), keyID, contentID, extension)
	path := filepath.Join(libraryDir, fileName)

	temp, err := os.CreateTemp(libraryDir, ".image-library-*")
	if err != nil {
		return GeneratedImage{}, fmt.Errorf("falha ao criar arquivo temporário da biblioteca: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return GeneratedImage{}, fmt.Errorf("falha ao escrever imagem da biblioteca: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return GeneratedImage{}, fmt.Errorf("falha ao sincronizar imagem da biblioteca: %w", err)
	}
	if err := temp.Close(); err != nil {
		return GeneratedImage{}, fmt.Errorf("falha ao fechar imagem da biblioteca: %w", err)
	}
	if err := publishImageFile(tempPath, path, contentHash); err != nil {
		return GeneratedImage{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = v.db.ExecContext(ctx, `INSERT INTO generated_image_cache
		(cache_key, name, prompt, extension, relative_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
		name = excluded.name,
		prompt = excluded.prompt,
		extension = excluded.extension,
		relative_path = excluded.relative_path,
		updated_at = excluded.updated_at`,
		key, name, prompt, extension, fileName, now, now,
	)
	if err != nil {
		return GeneratedImage{}, fmt.Errorf("falha ao registrar imagem na biblioteca: %w", err)
	}

	return GeneratedImage{
		Key: key, Name: name, Prompt: prompt, Extension: extension,
		RelativePath: fileName, Path: path,
	}, nil
}

func publishImageFile(tempPath, finalPath string, expectedHash [32]byte) error {
	if info, err := os.Lstat(finalPath); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("destino da biblioteca de imagens não é um arquivo regular")
		}
		if err := verifyImageHash(finalPath, expectedHash); err != nil {
			return err
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("falha ao inspecionar destino da biblioteca: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		// Another process may have published the same content after the initial stat.
		if _, statErr := os.Lstat(finalPath); statErr == nil {
			return verifyImageHash(finalPath, expectedHash)
		}
		return fmt.Errorf("falha ao publicar imagem na biblioteca: %w", err)
	}
	return nil
}

func verifyImageHash(path string, expected [32]byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("falha ao inspecionar imagem existente na biblioteca: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("arquivo existente da biblioteca não é regular")
	}
	actual, _, err := hashFile(path)
	if err != nil {
		return fmt.Errorf("falha ao verificar imagem existente na biblioteca: %w", err)
	}
	if actual != hex.EncodeToString(expected[:]) {
		return errors.New("arquivo existente da biblioteca não corresponde ao conteúdo esperado")
	}
	return nil
}

func imageLibrarySlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.Join(strings.Fields(name), "-")
	name = imageLibrarySlugPattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-_")
	if runes := []rune(name); len(runes) > 48 {
		name = string(runes[:48])
	}
	if name == "" {
		return "imagem"
	}
	return name
}
