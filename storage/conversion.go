package storage

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// ConversionStore persists rendered conversions. Implementations can write to
// a local cache directory (served by Caddy) or a CDN. URL is the public
// location a client should use for a key; a local store returns a path, while
// a CDN store returns an absolute URL.
type ConversionStore interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]ConversionEntry, error)
	Stats(ctx context.Context) (ConversionStats, error)
	Purge(ctx context.Context) (int, error)
	URL(key string) string
}

// ConversionEntry describes one stored conversion file.
type ConversionEntry struct {
	Key        string
	ByteSize   int64
	ModifiedAt time.Time
}

// ConversionStats is the total number and size of stored conversions.
type ConversionStats struct {
	Count int
	Bytes int64
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

// List returns the conversions stored under prefix (an asset id). A missing
// prefix is not an error; it simply has no conversions yet.
func (s *LocalConversions) List(_ context.Context, prefix string) ([]ConversionEntry, error) {
	dir, err := s.path(prefix)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]ConversionEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, ConversionEntry{
			Key:        path.Join(prefix, entry.Name()),
			ByteSize:   info.Size(),
			ModifiedAt: info.ModTime(),
		})
	}
	return out, nil
}

// Stats walks the conversion tree and returns the file count and total bytes.
func (s *LocalConversions) Stats(_ context.Context) (ConversionStats, error) {
	var stats ConversionStats
	err := filepath.WalkDir(s.baseDir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		stats.Count++
		stats.Bytes += info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return stats, nil
	}
	return stats, err
}

// Purge removes every stored conversion and returns the number of files that
// were deleted. The base directory itself is kept for the running server.
func (s *LocalConversions) Purge(_ context.Context) (int, error) {
	entries, err := os.ReadDir(s.baseDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		full := filepath.Join(s.baseDir, entry.Name())
		if entry.IsDir() {
			_ = filepath.WalkDir(full, func(_ string, item fs.DirEntry, err error) error {
				if err == nil && !item.IsDir() && !strings.HasPrefix(item.Name(), ".") {
					removed++
				}
				return nil
			})
		} else if !strings.HasPrefix(entry.Name(), ".") {
			removed++
		}
		if err := os.RemoveAll(full); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func (s *LocalConversions) URL(key string) string {
	clean := strings.TrimLeft(filepath.ToSlash(key), "/")
	if s.baseURL == "" {
		return "/" + clean
	}
	return s.baseURL + "/" + clean
}
