-- +goose Up
ALTER TABLE attributes_metadata ADD COLUMN unassigned_label TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE attributes_metadata DROP COLUMN unassigned_label;
