-- +goose Up
ALTER TABLE assets ADD COLUMN page_count INTEGER;

-- +goose Down
-- SQLite doesn't support DROP COLUMN in older versions; omitted for compatibility