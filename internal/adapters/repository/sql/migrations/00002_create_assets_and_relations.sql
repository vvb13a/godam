-- +goose Up
CREATE TABLE IF NOT EXISTS assets (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    width INTEGER,
    height INTEGER,
    duration_sec REAL,
    raw_metadata TEXT,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS asset_collections (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, collection_id)
);

CREATE INDEX IF NOT EXISTS idx_asset_collections_col ON asset_collections(collection_id);

-- +goose Down
DROP TABLE IF EXISTS asset_collections;
DROP TABLE IF EXISTS assets;