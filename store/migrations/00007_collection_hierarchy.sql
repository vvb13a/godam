-- +goose Up
-- Collections become a hierarchy (at most one parent) and gain a per-folder
-- color. The free-form icon is dropped in favour of a folder icon tinted with
-- the color. `sort` orders siblings.

ALTER TABLE collections ADD COLUMN parent_id TEXT REFERENCES collections(id) ON DELETE CASCADE;
ALTER TABLE collections ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE collections ADD COLUMN sort INTEGER NOT NULL DEFAULT 0;

ALTER TABLE collections DROP COLUMN icon;

CREATE INDEX idx_collections_parent ON collections(parent_id);

-- +goose Down
DROP INDEX IF EXISTS idx_collections_parent;
ALTER TABLE collections ADD COLUMN icon TEXT NOT NULL DEFAULT '';
-- SQLite cannot cheaply drop the added columns; parent_id/color/sort are left.
