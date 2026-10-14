-- +goose Up
-- Authentication: users own libraries; sessions back cookie auth. Pre-auth
-- libraries are adopted by a seeded developer user so existing installs remain
-- reachable through the developer login.

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

-- Developer user (no password; reachable through the developer login button).
INSERT INTO users (id, name, email, password_hash, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Yi Sun-sin',
    'dev@godam.local',
    '',
    '2020-01-01T00:00:00Z',
    '2020-01-01T00:00:00Z'
);

ALTER TABLE libraries ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE CASCADE;
UPDATE libraries SET user_id = '00000000-0000-0000-0000-000000000001';

ALTER TABLE assets ADD COLUMN uploaded_by TEXT REFERENCES users(id) ON DELETE SET NULL;
UPDATE assets SET uploaded_by = '00000000-0000-0000-0000-000000000001';

CREATE INDEX idx_libraries_user ON libraries(user_id);
CREATE INDEX idx_assets_uploaded_by ON assets(uploaded_by);

-- +goose Down
DROP INDEX IF EXISTS idx_assets_uploaded_by;
DROP INDEX IF EXISTS idx_libraries_user;
-- SQLite cannot drop columns on older versions; the added columns are left behind.
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
