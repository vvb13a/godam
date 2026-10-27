-- +goose Up
ALTER TABLE instance ADD COLUMN attribute_value_search INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE instance DROP COLUMN attribute_value_search;
