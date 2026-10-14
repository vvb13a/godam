-- +goose Up
-- Smart collections are saved filter presets: a name, a colour and a JSON
-- filter (collection/tag/type/sort) that is re-applied when selected. They are
-- flat (no hierarchy) and scoped to a library.

CREATE TABLE smart_collections (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    filter TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE (library_id, name)
);

CREATE INDEX idx_smart_collections_library ON smart_collections(library_id);

-- +goose Down
DROP TABLE IF EXISTS smart_collections;
