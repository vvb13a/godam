-- +goose Up
-- Authentication is JWT-only (or trusted direct access). Remove the local
-- user/session tables and the user-id attribution columns. uploaded_by_name is
-- kept as an optional display value supplied by a token.
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

DROP INDEX IF EXISTS idx_assets_uploaded_by;
ALTER TABLE assets DROP COLUMN uploaded_by;

DROP INDEX IF EXISTS idx_presets_added_by;
ALTER TABLE presets DROP COLUMN added_by;

-- +goose Down
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE INDEX idx_sessions_user ON sessions(user_id);

ALTER TABLE assets ADD COLUMN uploaded_by TEXT;
CREATE INDEX idx_assets_uploaded_by ON assets(uploaded_by);

ALTER TABLE presets ADD COLUMN added_by TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_presets_added_by ON presets(added_by);
