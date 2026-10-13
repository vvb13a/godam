-- +goose Up
CREATE TABLE tag_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    color TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

ALTER TABLE tags ADD COLUMN group_id TEXT REFERENCES tag_groups(id) ON DELETE SET NULL;
ALTER TABLE tags ADD COLUMN starred INTEGER NOT NULL DEFAULT 0;

-- +goose Down
DROP TABLE tag_groups;
-- SQLite cannot drop columns on older versions; tag columns are left behind.
