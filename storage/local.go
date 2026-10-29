package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// Local stores blobs on the local filesystem under a base directory.
type Local struct {
	baseDir string
}

func NewLocal(baseDir string) (*Local, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}
	return &Local{baseDir: baseDir}, nil
}

func (s *Local) Save(_ context.Context, key string, r io.Reader) error {
	destPath := filepath.Join(s.baseDir, key)
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	out, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	tmpName := out.Name()
	if _, err := io.Copy(out, r); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (s *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.baseDir, key))
}

func (s *Local) Delete(_ context.Context, key string) error {
	return os.Remove(filepath.Join(s.baseDir, key))
}
