-- +goose Up
-- Per-library SFTP backup configuration plus the replicas and queued jobs that
-- keep the remote copy in sync with the primary (local) storage.

CREATE TABLE library_backups (
  library_id TEXT PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
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
  library_id TEXT NOT NULL,
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
DROP TABLE library_backups;
