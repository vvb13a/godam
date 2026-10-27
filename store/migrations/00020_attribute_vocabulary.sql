-- +goose Up
ALTER TABLE attributes_metadata ADD COLUMN values_locked INTEGER NOT NULL DEFAULT 0;
ALTER TABLE instance ADD COLUMN upload_attribute_keys INTEGER NOT NULL DEFAULT 1;
ALTER TABLE instance ADD COLUMN upload_attribute_values INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE instance DROP COLUMN upload_attribute_values;
ALTER TABLE instance DROP COLUMN upload_attribute_keys;
ALTER TABLE attributes_metadata DROP COLUMN values_locked;
