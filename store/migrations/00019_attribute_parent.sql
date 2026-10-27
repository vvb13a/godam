-- +goose Up
ALTER TABLE attributes_metadata ADD COLUMN parent_key TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE attributes_metadata DROP COLUMN parent_key;
