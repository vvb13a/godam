-- +goose Up
-- Tags gain a freeform description and a modification timestamp, matching
-- collections so the Inspector can present (and edit) both the same way.
-- Existing rows are backfilled from created_at.

ALTER TABLE tags ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE tags ADD COLUMN updated_at DATETIME;
UPDATE tags SET updated_at = created_at WHERE updated_at IS NULL;

-- +goose Down
-- SQLite cannot drop columns on older versions; the added columns are left behind.
