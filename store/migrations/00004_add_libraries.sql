-- +goose Up
-- Introduce Library as the top-level tenant scope. This is a breaking schema
-- change: existing data has no library and there is no sensible default, so the
-- old tables are dropped and recreated with library-scoped ownership. Name
-- uniqueness becomes per-library (library_id, name) instead of global.

DROP TABLE IF EXISTS asset_tags;
DROP TABLE IF EXISTS asset_collections;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS tag_groups;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS assets;

CREATE TABLE libraries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE collections (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    icon TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE (library_id, name)
);

CREATE TABLE assets (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
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

CREATE TABLE tag_groups (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    color TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE (library_id, name)
);

CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    group_id TEXT REFERENCES tag_groups(id) ON DELETE SET NULL,
    starred INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    UNIQUE (library_id, name)
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

CREATE INDEX idx_assets_library ON assets(library_id);
CREATE INDEX idx_tags_library ON tags(library_id);
CREATE INDEX idx_tag_groups_library ON tag_groups(library_id);
CREATE INDEX idx_collections_library ON collections(library_id);

-- +goose Down
DROP TABLE IF EXISTS asset_tags;
DROP TABLE IF EXISTS asset_collections;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS tag_groups;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS libraries;
