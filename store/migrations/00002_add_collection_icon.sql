-- +goose Up
ALTER TABLE collections ADD COLUMN icon TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite cannot drop columns on older versions; omitted.
