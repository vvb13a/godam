-- +goose Up
-- Reserve "preview" for the 400px library image. Stored poster/waveform assets
-- are visual sources for /img, and image_of points at their parent asset.
UPDATE assets SET role = 'visual' WHERE role = 'preview';
UPDATE assets
SET filename = replace(filename, '-preview.', '-visual.')
WHERE role = 'visual';

ALTER TABLE assets RENAME COLUMN preview_of TO image_of;

DROP INDEX IF EXISTS idx_assets_preview_of;
CREATE INDEX idx_assets_image_of ON assets(image_of);

-- +goose Down
DROP INDEX IF EXISTS idx_assets_image_of;
ALTER TABLE assets RENAME COLUMN image_of TO preview_of;
CREATE INDEX idx_assets_preview_of ON assets(preview_of);

UPDATE assets SET role = 'preview' WHERE role = 'visual';
UPDATE assets
SET filename = replace(filename, '-visual.', '-preview.')
WHERE role = 'preview';
