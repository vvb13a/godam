-- +goose Up
-- Collections were replaced by attributes. Presets are now library-wide, so
-- their collection scoping and the library-wide flag are no longer needed.
DROP TABLE IF EXISTS preset_collections;
DROP TABLE IF EXISTS asset_collections;
DROP TABLE IF EXISTS collections;
ALTER TABLE presets DROP COLUMN library_wide;

-- +goose Down
ALTER TABLE presets ADD COLUMN library_wide INTEGER NOT NULL DEFAULT 1;

CREATE TABLE collections (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES collections(id) ON DELETE CASCADE,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    color TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_collections_parent ON collections(parent_id);

CREATE TABLE asset_collections (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, collection_id)
);

CREATE INDEX idx_asset_collections_col ON asset_collections(collection_id);

CREATE TABLE preset_collections (
    preset_id TEXT NOT NULL REFERENCES presets(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (preset_id, collection_id)
);

CREATE INDEX idx_preset_collections_collection ON preset_collections(collection_id);
