-- +goose Up
ALTER TABLE assets ADD COLUMN storage_driver TEXT NOT NULL DEFAULT 'local';

-- +goose Down