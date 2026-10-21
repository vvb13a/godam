-- +goose Up
-- Derived preview images (video posters, PDF pages, audio waveforms) are real
-- assets so the same delivery code can resolve them via preview_of.
ALTER TABLE assets ADD COLUMN role TEXT NOT NULL DEFAULT 'original';
ALTER TABLE assets ADD COLUMN preview_of TEXT;

CREATE INDEX idx_assets_role ON assets(role);
CREATE INDEX idx_assets_preview_of ON assets(preview_of);

-- +goose Down
DROP INDEX idx_assets_preview_of;
DROP INDEX idx_assets_role;
ALTER TABLE assets DROP COLUMN preview_of;
ALTER TABLE assets DROP COLUMN role;
