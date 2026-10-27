-- +goose Up
ALTER TABLE attributes_metadata ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE attributes_metadata ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE attributes_metadata ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE attributes_metadata DROP COLUMN position;
ALTER TABLE attributes_metadata DROP COLUMN color;
ALTER TABLE attributes_metadata DROP COLUMN description;
