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
