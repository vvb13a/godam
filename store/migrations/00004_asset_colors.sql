-- +goose Up
CREATE TABLE asset_colors (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    color TEXT NOT NULL,
    bucket TEXT NOT NULL,
    percentage REAL NOT NULL,
    PRIMARY KEY (asset_id, position)
);

CREATE INDEX idx_asset_colors_bucket ON asset_colors(bucket);

-- +goose Down
DROP TABLE IF EXISTS asset_colors;
