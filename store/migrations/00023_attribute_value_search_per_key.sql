-- +goose Up
ALTER TABLE attributes_metadata ADD COLUMN value_search INTEGER NOT NULL DEFAULT 0;
ALTER TABLE instance DROP COLUMN attribute_value_search;

-- +goose Down
ALTER TABLE instance ADD COLUMN attribute_value_search INTEGER NOT NULL DEFAULT 0;
ALTER TABLE attributes_metadata DROP COLUMN value_search;
