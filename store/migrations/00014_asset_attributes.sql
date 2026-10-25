-- +goose Up
-- Dynamic, host-owned attributes. An asset may have multiple values for the
-- same key. The (key, value, asset_id) index backs attribute filtering.
CREATE TABLE asset_attributes (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (asset_id, key, value)
);

CREATE INDEX idx_asset_attributes_key_value ON asset_attributes(key, value, asset_id);
CREATE INDEX idx_asset_attributes_asset ON asset_attributes(asset_id);

-- +goose Down
DROP TABLE asset_attributes;
