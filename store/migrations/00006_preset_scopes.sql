-- +goose Up
-- A preset is either library-wide or available through collection/tag
-- membership. Existing presets stay library-wide.
ALTER TABLE presets ADD COLUMN library_wide INTEGER NOT NULL DEFAULT 1;

CREATE TABLE preset_collections (
    preset_id TEXT NOT NULL REFERENCES presets(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (preset_id, collection_id)
);

CREATE INDEX idx_preset_collections_collection ON preset_collections(collection_id);

CREATE TABLE preset_tags (
    preset_id TEXT NOT NULL REFERENCES presets(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (preset_id, tag_id)
);

CREATE INDEX idx_preset_tags_tag ON preset_tags(tag_id);

-- +goose Down
DROP TABLE preset_tags;
DROP TABLE preset_collections;
ALTER TABLE presets DROP COLUMN library_wide;
