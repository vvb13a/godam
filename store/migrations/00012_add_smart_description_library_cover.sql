-- +goose Up
-- Smart collections gain a freeform description so the Inspector can edit them
-- like collections. Libraries gain an optional cover asset whose preview is
-- shown in the default Inspector; it clears when the asset is deleted.

ALTER TABLE smart_collections ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE libraries ADD COLUMN cover_asset_id TEXT;

-- +goose Down
-- SQLite cannot drop columns on older versions; the added columns are left behind.
