package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ConversionStore persists rendered conversions. Implementations can write to
// a local cache directory (served by Caddy) or a CDN. URL is the public
// location a client should use for a key; a local store returns a path, while
// a CDN store returns an absolute URL.
type ConversionStore interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

// LocalConversions stores conversions on the local filesystem under a base
// directory. Writes are atomic (temp file plus rename), so Caddy never serves a
// partially-written conversion.
type LocalConversions struct {
	baseDir string
	baseURL string
}

func NewLocalConversions(baseDir, baseURL string) (*LocalConversions, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}
	return &LocalConversions{
		baseDir: baseDir,
		baseURL: strings.TrimRight(baseURL, "/"),
	}, nil
}

// path resolves a key inside baseDir, rejecting absolute paths and traversal.
func (s *LocalConversions) path(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) ||
		strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid conversion key %q", key)
	}
	return filepath.Join(s.baseDir, clean), nil
}

func (s *LocalConversions) Save(_ context.Context, key string, r io.Reader) error {
	dest, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".conversion-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// The cache is read by Caddy from another container; keep it world-readable
	// while still relying on the volume for isolation.
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (s *LocalConversions) Open(_ context.Context, key string) (io.ReadCloser, error) {
	file, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(file)
}

func (s *LocalConversions) Delete(_ context.Context, key string) error {
	file, err := s.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(file)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *LocalConversions) URL(key string) string {
	clean := strings.TrimLeft(filepath.ToSlash(key), "/")
	if s.baseURL == "" {
		return "/" + clean
	}
	return s.baseURL + "/" + clean
}
