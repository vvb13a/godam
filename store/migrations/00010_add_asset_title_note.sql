-- +goose Up
-- Editable presentation fields for an asset. `title` is a human-friendly name
-- that falls back to the filename when empty; `note` is freeform text. The
-- `updated_at` column tracks the last change to these fields (existing rows are
-- backfilled from created_at).

ALTER TABLE assets ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE assets ADD COLUMN note TEXT NOT NULL DEFAULT '';
ALTER TABLE assets ADD COLUMN updated_at DATETIME;
UPDATE assets SET updated_at = created_at WHERE updated_at IS NULL;

-- +goose Down
-- SQLite cannot drop columns on older versions; the added columns are left behind.
