-- +goose Up
-- Clean single-instance schema. One godam process serves exactly one library.
-- The database and storage volume are the isolation boundary; no library_id
-- columns exist on any row.

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

CREATE TABLE instance (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '8b5cf6',
    cover_asset_id TEXT,
    watermark_asset_id TEXT,
    allowed_mime_types TEXT NOT NULL DEFAULT '',
    max_upload_bytes INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE assets (
    id TEXT PRIMARY KEY,
    uploaded_by TEXT,
    filename TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    storage_driver TEXT NOT NULL DEFAULT 'local',
    storage_key TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    width INTEGER,
    height INTEGER,
    duration_sec REAL,
    page_count INTEGER,
    raw_metadata TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_assets_uploaded_by ON assets(uploaded_by);

CREATE TABLE collections (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES collections(id) ON DELETE CASCADE,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    color TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_collections_parent ON collections(parent_id);

CREATE TABLE tag_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    color TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    group_id TEXT REFERENCES tag_groups(id) ON DELETE SET NULL,
    starred INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, tag_id)
);

CREATE INDEX idx_asset_tags_tag ON asset_tags(tag_id);

CREATE TABLE asset_collections (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, collection_id)
);

CREATE INDEX idx_asset_collections_col ON asset_collections(collection_id);

CREATE TABLE smart_collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    filter TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE backup_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 0,
    driver TEXT NOT NULL DEFAULT 'sftp',
    config TEXT NOT NULL DEFAULT '',
    last_run_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE asset_replicas (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    destination TEXT NOT NULL DEFAULT 'sftp',
    status TEXT NOT NULL DEFAULT 'pending',
    storage_key TEXT NOT NULL DEFAULT '',
    byte_size INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (asset_id, destination)
);

CREATE INDEX idx_asset_replicas_status ON asset_replicas(destination, status);

CREATE TABLE storage_jobs (
    id TEXT PRIMARY KEY,
    asset_id TEXT NOT NULL DEFAULT '',
    storage_driver TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL,
    action TEXT NOT NULL,
    destination TEXT NOT NULL DEFAULT 'sftp',
    attempts INTEGER NOT NULL DEFAULT 0,
    run_after DATETIME NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_storage_jobs_run_after ON storage_jobs(run_after);

-- +goose Down
DROP TABLE storage_jobs;
DROP TABLE asset_replicas;
DROP TABLE backup_settings;
DROP TABLE smart_collections;
DROP TABLE asset_collections;
DROP TABLE asset_tags;
DROP TABLE tags;
DROP TABLE tag_groups;
DROP TABLE collections;
DROP TABLE assets;
DROP TABLE instance;
DROP TABLE sessions;
DROP TABLE users;
