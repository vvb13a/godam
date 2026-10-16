-- +goose Up
-- Per-library upload policy. `allowed_mime_types` is a JSON array of MIME
-- types (supporting "type/*" wildcards); an empty array means "use the global
-- default". `max_upload_bytes` of 0 means "use the global default".

ALTER TABLE libraries ADD COLUMN allowed_mime_types TEXT NOT NULL DEFAULT '';
ALTER TABLE libraries ADD COLUMN max_upload_bytes INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite cannot drop columns on older versions; the added columns are left behind.
