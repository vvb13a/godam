package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// dbFileName is the SQLite database file name.
const dbFileName = "godam.db"

// DataDir returns the per-user directory that holds the SQLite database and
// the local asset storage. It uses the platform's data location under a
// "godam" folder:
//
//	macOS   ~/Library/Application Support/godam
//	Linux   $XDG_DATA_HOME/godam (~/.local/share/godam)
//	Windows %LOCALAPPDATA%\godam
func DataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "godam"), nil
		}
		return "", errors.New("LOCALAPPDATA is not set")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "godam"), nil
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return filepath.Join(dir, "godam"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, ".local", "share", "godam"), nil
	}
}

// DatabasePath returns the full path of the SQLite database file, creating its
// parent directory if needed.
func DatabasePath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	return filepath.Join(dir, dbFileName), nil
}

// StorageDir returns the directory that holds the uploaded assets, creating it
// if needed. It lives next to the database inside the per-user data directory.
func StorageDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "storage")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("create storage dir: %w", err)
	}
	return path, nil
}
