-- +goose Up
-- Presentation metadata for host-owned attribute keys and values. Labels are
-- display overrides only; the underlying key/value strings stay unchanged.
-- Value is empty for key-level rows. Sort is only meaningful for key rows.
CREATE TABLE attributes_metadata (
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL DEFAULT '',
    sort TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (key, value)
);

CREATE INDEX idx_attributes_metadata_key ON attributes_metadata(key);

-- +goose Down
DROP INDEX IF EXISTS idx_attributes_metadata_key;
DROP TABLE IF EXISTS attributes_metadata;
