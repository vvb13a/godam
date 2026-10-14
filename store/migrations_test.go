package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

// TestLibraryUsersMigration upgrades a populated pre-auth-share schema to the
// many-to-many head and checks that the old single owner was backfilled into
// library_users and that the redundant column is gone.
func TestLibraryUsersMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrationsFS)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.UpTo(sqlDB, "migrations", 5); err != nil {
		t.Fatalf("migrate to 5: %v", err)
	}

	if _, err := sqlDB.Exec(
		`INSERT INTO users (id, name, email, password_hash, created_at, updated_at)
		 VALUES ('u1', 'User', 'u1@example.com', '', '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := sqlDB.Exec(
		`INSERT INTO libraries (id, name, color, sort, created_at, updated_at, user_id)
		 VALUES ('l1', 'Lib', '', 0, '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z', 'u1')`,
	); err != nil {
		t.Fatalf("seed library: %v", err)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	var members int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM library_users WHERE library_id = 'l1' AND user_id = 'u1'`,
	).Scan(&members); err != nil {
		t.Fatalf("query membership: %v", err)
	}
	if members != 1 {
		t.Fatalf("expected backfilled membership, got %d", members)
	}

	// The legacy column must be gone: a plain library insert must succeed.
	if _, err := sqlDB.Exec(
		`INSERT INTO libraries (id, name, color, sort, created_at, updated_at)
		 VALUES ('l2', 'Lib2', '', 1, '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("insert after drop column: %v", err)
	}
}

// TestCollectionHierarchyMigration upgrades a populated v6 schema to the
// hierarchical collection head: parent_id/color/sort appear and the icon column
// is dropped.
func TestCollectionHierarchyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrationsFS)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.UpTo(sqlDB, "migrations", 6); err != nil {
		t.Fatalf("migrate to 6: %v", err)
	}

	if _, err := sqlDB.Exec(
		`INSERT INTO libraries (id, name, color, sort, created_at, updated_at)
		 VALUES ('lib', 'Lib', '', 0, '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("seed library: %v", err)
	}
	if _, err := sqlDB.Exec(
		`INSERT INTO collections (id, library_id, name, description, icon, created_at, updated_at)
		 VALUES ('c1', 'lib', 'Old', '', 'folder', '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("seed collection: %v", err)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	// icon is gone and the new columns exist with sane defaults.
	if _, err := sqlDB.Exec(
		`INSERT INTO collections (id, library_id, parent_id, name, color, sort, created_at, updated_at)
		 VALUES ('c2', 'lib', 'c1', 'Child', '', 0, '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("insert with hierarchy columns: %v", err)
	}
	var parent string
	if err := sqlDB.QueryRow(`SELECT parent_id FROM collections WHERE id = 'c2'`).Scan(&parent); err != nil {
		t.Fatalf("query parent: %v", err)
	}
	if parent != "c1" {
		t.Fatalf("expected parent c1, got %q", parent)
	}
}
