-- +goose Up
-- Views are now client-side filter snapshots stored in localStorage.
DROP TABLE IF EXISTS smart_collections;

-- +goose Down
CREATE TABLE smart_collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    filter TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
