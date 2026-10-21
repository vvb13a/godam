-- +goose Up
-- Per-category upload size limits, stored as JSON. Zero means unlimited.
ALTER TABLE instance ADD COLUMN upload_limits TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE instance DROP COLUMN upload_limits;
