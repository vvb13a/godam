-- +goose Up
-- Ground-zero schema. This is the complete schema as of the goose migration
-- baseline; there are no incremental migrations before it.

CREATE TABLE collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE assets (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    storage_driver TEXT NOT NULL DEFAULT 'local',
    storage_key TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    width INTEGER,
    height INTEGER,
    duration_sec REAL,
    page_count INTEGER,
    raw_metadata TEXT,
    created_at DATETIME NOT NULL
);

CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL
);

CREATE TABLE asset_collections (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, collection_id)
);

CREATE INDEX idx_asset_collections_col ON asset_collections(collection_id);

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, tag_id)
);

CREATE INDEX idx_asset_tags_tag ON asset_tags(tag_id);

-- +goose Down
DROP TABLE asset_tags;
DROP TABLE asset_collections;
DROP TABLE tags;
DROP TABLE assets;
DROP TABLE collections;
