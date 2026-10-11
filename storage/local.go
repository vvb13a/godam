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
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)
	return err
}

func (s *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.baseDir, key))
}

func (s *Local) Delete(_ context.Context, key string) error {
	return os.Remove(filepath.Join(s.baseDir, key))
}
