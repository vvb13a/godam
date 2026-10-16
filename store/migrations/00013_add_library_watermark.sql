-- +goose Up
-- Optional library-wide default watermark asset, offered inside the image
-- editor's watermark tool. Cleared when the asset is deleted.

ALTER TABLE libraries ADD COLUMN watermark_asset_id TEXT;

-- +goose Down
-- SQLite cannot drop columns on older versions; the added column is left behind.
