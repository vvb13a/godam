-- +goose Up
-- Persist the display name supplied by a JWT embed token. Session uploads can
-- leave this empty and continue resolving the name from the users table.
ALTER TABLE assets ADD COLUMN uploaded_by_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE assets DROP COLUMN uploaded_by_name;
