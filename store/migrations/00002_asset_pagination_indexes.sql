-- +goose Up
-- Keyset pagination and server-side filtering indexes.

CREATE INDEX IF NOT EXISTS idx_assets_created_at_id
    ON assets(created_at, id);

CREATE INDEX IF NOT EXISTS idx_assets_updated_at_id
    ON assets(updated_at, id);

CREATE INDEX IF NOT EXISTS idx_assets_filename_nocase_id
    ON assets(filename COLLATE NOCASE, id);

CREATE INDEX IF NOT EXISTS idx_assets_byte_size_id
    ON assets(byte_size, id);

CREATE INDEX IF NOT EXISTS idx_assets_mime_type
    ON assets(mime_type);

-- +goose Down
DROP INDEX IF EXISTS idx_assets_mime_type;
DROP INDEX IF EXISTS idx_assets_byte_size_id;
DROP INDEX IF EXISTS idx_assets_filename_nocase_id;
DROP INDEX IF EXISTS idx_assets_updated_at_id;
DROP INDEX IF EXISTS idx_assets_created_at_id;
