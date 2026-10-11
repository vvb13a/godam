package store

import (
	"path/filepath"
	"testing"
)

// TestDataDirEndsWithAppFolder pins the per-OS data directory to the "godam"
// app folder, the same convention goaudit uses for its database.
func TestDataDirEndsWithAppFolder(t *testing.T) {
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if got := filepath.Base(dir); got != "godam" {
		t.Errorf("DataDir base = %q, want %q", got, "godam")
	}
}

// TestDatabasePathInDataDir verifies the SQLite database lives directly inside
// the per-OS data directory.
func TestDatabasePathInDataDir(t *testing.T) {
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	dbPath, err := DatabasePath()
	if err != nil {
		t.Fatalf("DatabasePath: %v", err)
	}
	if got := filepath.Dir(dbPath); got != dir {
		t.Errorf("database dir = %q, want %q", got, dir)
	}
	if got := filepath.Base(dbPath); got != dbFileName {
		t.Errorf("database file = %q, want %q", got, dbFileName)
	}
}

// TestStorageDirInDataDir verifies uploaded assets share the per-OS data
// directory with the database.
func TestStorageDirInDataDir(t *testing.T) {
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	storagePath, err := StorageDir()
	if err != nil {
		t.Fatalf("StorageDir: %v", err)
	}
	if got := filepath.Dir(storagePath); got != dir {
		t.Errorf("storage dir = %q, want %q", got, dir)
	}
}
