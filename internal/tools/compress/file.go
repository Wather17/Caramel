package compress

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileResult struct {
	Input, Output                  string
	OriginalBytes, CompressedBytes int64
	State, Reason                  string
	Format                         string
}
type BatchReport struct {
	Items                                      []FileResult
	OriginalBytes, CompressedBytes, SavedBytes int64
}

func CompressImageFile(inputPath, outputPath string, opts ResolvedOptions) (FileResult, error) {
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return FileResult{}, err
	}
	r, err := CompressImageBytes(b, opts)
	if err != nil {
		return FileResult{}, err
	}
	fr := FileResult{Input: inputPath, OriginalBytes: int64(len(b)), State: "compressed", Reason: r.Reason, Format: r.Format}
	if r.Skipped {
		fr.State = "skipped"
		return fr, nil
	}
	if outputPath == "" {
		outputPath = filepath.Join(filepath.Dir(inputPath), strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))+"_compressed"+r.Extension)
	}
	fr.Output = outputPath
	fr.CompressedBytes = int64(len(r.Data))
	if err := writeAtomic(outputPath, r.Data); err != nil {
		return FileResult{}, err
	}
	return fr, nil
}

func CompressImageFileInPlace(inputPath string, opts ResolvedOptions) (FileResult, error) {
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return FileResult{}, err
	}
	r, err := CompressImageBytes(b, opts)
	fr := FileResult{Input: inputPath, OriginalBytes: int64(len(b)), State: "compressed"}
	if err != nil {
		return FileResult{}, err
	}
	if r.Skipped {
		fr.State = "skipped"
		fr.Reason = r.Reason
		return fr, nil
	}
	mode := os.FileMode(0644)
	if info, e := os.Stat(inputPath); e == nil {
		mode = info.Mode().Perm()
	}
	target := inputPath
	if filepath.Ext(inputPath) != r.Extension {
		target = strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + r.Extension
		if _, e := os.Lstat(target); e == nil {
			return FileResult{}, fmt.Errorf("destino já existe: %s", target)
		} else if !os.IsNotExist(e) {
			return FileResult{}, e
		}
	}
	var writeErr error
	if target == inputPath {
		writeErr = replaceAtomic(target, r.Data, mode)
	} else {
		writeErr = writeAtomicMode(target, r.Data, mode)
	}
	if writeErr != nil {
		return FileResult{}, writeErr
	}
	if target != inputPath {
		if err := os.Remove(inputPath); err != nil {
			_ = os.Remove(target)
			return FileResult{}, err
		}
	}
	fr.Output = target
	fr.CompressedBytes = int64(len(r.Data))
	fr.Format = r.Format
	return fr, nil
}

func replaceAtomic(path string, data []byte, mode os.FileMode) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".compress-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Chmod(mode); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}

func writeAtomic(path string, data []byte) error {
	return writeAtomicMode(path, data, 0644)
}
func writeAtomicMode(path string, data []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("destino já existe: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".compress-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Link(tmp, path); err != nil {
		return fmt.Errorf("publicar destino %s: %w", path, err)
	}
	return os.Remove(tmp)
}

func WriteImageBytes(path string, data []byte) error { return writeAtomic(path, data) }
