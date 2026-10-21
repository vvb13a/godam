-- +goose Up
-- Replace the library-wide upload limit with explicit per-type limits. Rows
-- that never set per-type limits inherit the previous 100 MB app default.
UPDATE instance
SET upload_limits = '{"image_bytes":104857600,"audio_bytes":104857600,"video_bytes":104857600,"pdf_bytes":104857600}'
WHERE upload_limits = ''
   OR upload_limits = '{"image_bytes":0,"audio_bytes":0,"video_bytes":0,"pdf_bytes":0}';

ALTER TABLE instance DROP COLUMN max_upload_bytes;

-- +goose Down
ALTER TABLE instance ADD COLUMN max_upload_bytes INTEGER NOT NULL DEFAULT 0;
UPDATE instance SET max_upload_bytes = 104857600;
