package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

// TestInitialSchema checks that the clean single-instance baseline applies and
// contains no library scoping.
func TestInitialSchema(t *testing.T) {
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
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	rows, err := sqlDB.Query(`PRAGMA table_info(assets)`)
	if err != nil {
		t.Fatalf("table info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		if name == "library_id" {
			t.Fatal("assets must not have a library_id column")
		}
		if name == "uploaded_by" {
			t.Fatal("assets must not have an uploaded_by column")
		}
	}

	presetRows, err := sqlDB.Query(`PRAGMA table_info(presets)`)
	if err != nil {
		t.Fatalf("preset table info: %v", err)
	}
	defer presetRows.Close()
	for presetRows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := presetRows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan preset column: %v", err)
		}
		if name == "added_by" {
			t.Fatal("presets must not have an added_by column")
		}
	}

	if _, err := sqlDB.Exec(`INSERT INTO instance (id, name, created_at, updated_at) VALUES ('i1', 'Test', '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert instance: %v", err)
	}
}
