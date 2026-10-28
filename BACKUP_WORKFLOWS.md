# Backup, Export & Recovery Workflows

This document describes the three backup/transfer workflows for godam, what is
already implemented, and how to add the remaining pieces.

> Since the single-instance refactor, the instance and the library are the same
> thing. The current code uses `backup_settings` and has no `library_id` columns;
> older references below to per-library behavior mean per-instance behavior.

The guiding principle is:

> The SQLite database is the catalog. Storage files alone are not a portable
> import; the database owns asset IDs, storage keys, metadata, attributes, and
> libraries.

There are three distinct workflows:

1. **Selective asset export/import** — ZIP + manifest, for moving assets
   between libraries or godam instances.
2. **Continuous backup** — continuous replication of storage and (optionally)
   the database, for disaster recovery and point-in-time recovery.
3. **Full snapshot** — a point-in-time archive of the database + storage +
   manifest, for migration or complete recovery.

Continuous backup has two levels:

- **v1 (implemented):** per-instance SFTP mirror of originals.
- **v2 (target):** versioned storage stream + lightweight DB snapshots, giving
  point-in-time recovery without re-zipping storage on every snapshot.

v2 is not a fourth workflow; it is a better implementation of workflow 2.

---

## Current state

### Already implemented

- Per-instance SFTP backup settings, encrypted credentials, status UI, async
  jobs, retries, and reconciliation:
  - `store/migrations/00015_library_backups.sql`
  - `service/backup_service.go`
  - `service/secretbox.go`
  - `api/backups.go`
  - `frontend/src/components/LibraryBackupDialog.vue`
- Bulk asset download as ZIP (originals, current filenames only):
  - `GET /api/assets/zip?ids=...` in `api/assets.go`
- Streaming uploads with chunked sessions for large files:
  - `POST /api/assets/uploads`, `PUT /api/assets/uploads/{id}/chunks`,
    `POST /api/assets/uploads/{id}/complete` in `api/assets.go`
- Local primary storage and preview storage:
  - originals: `storage/uploads/{assetID}{ext}`
  - previews: `storage/previews/{assetID}.{png|jpg}`
- SQLite in WAL mode, with migrations applied on startup.

### Important constraints

- `assets.storage_key` is relative to the local storage root.
- Preview files are derived and can be regenerated.
- `backup.key` (used to decrypt SFTP credentials) lives in the app data
  directory, not the storage directory.
- The current SFTP backup mirrors the latest original at a mutable key and
  deletes on asset deletion. It is **not** point-in-time safe.

---

## Workflow 1 — Selective asset export/import (ZIP + manifest)

### Use cases

- Move a selection of assets from one library to another.
- Hand assets to another godam instance (same or compatible version).
- Bulk download assets for use outside godam.

### Target format

```text
godam-assets.zip
  manifest.json
  assets/
    <asset-id>/
      <original-filename>
```

`manifest.json` should include:

```json
{
  "format": "godam.asset-export",
  "format_version": 1,
  "created_at": "2026-01-01T00:00:00Z",
  "source": {
    "godam_version": "x.y.z",
    "schema_version": 15,
    "library_id": "...",
    "library_name": "..."
  },
  "assets": [
    {
      "id": "...",
      "filename": "image.jpg",
      "mime_type": "image/jpeg",
      "byte_size": 12345,
      "sha256": "...",
      "archive_path": "assets/.../image.jpg",
      "title": "...",
      "note": "...",
      "attributes": { "tenant": ["acme"] }
    }
  ]
}
```

### Export

Add an endpoint or extend the existing ZIP endpoint:

- `GET /api/assets/zip?ids=...` — keep for plain downloads.
- `GET /api/assets/export?ids=...` — ZIP with `manifest.json`.
- Optionally `GET /api/libraries/{id}/export` for a whole library.

Implementation notes:

- Stream the ZIP; do not buffer in memory.
- Use `zip.Store` for already-compressed media, `zip.Deflate` for text.
- Use ZIP64 (Go's `archive/zip` handles it automatically).
- Include SHA-256 per asset in the manifest.
- Make archive entry names unique and safe (sanitize path separators).

### Import

Add `POST /api/libraries/{id}/import` accepting an uploaded ZIP:

1. Read and validate `manifest.json` first.
2. Refuse archives with a newer `format_version` or incompatible schema.
3. For each asset:
   - Verify SHA-256 and size.
   - Create a new asset ID (do not reuse the source ID unless importing into an
     empty/identical instance and explicit ID preservation is requested).
   - Store the original through the normal upload path so previews, metadata,
     and backup jobs are generated consistently.
4. Return a report: imported, skipped, failed, and why.
5. Make the import idempotent where possible (checksum + filename + library).

Do not import storage files directly into `storage/uploads/` without creating
asset rows — that produces orphan files and no catalog metadata.

### Acceptance criteria

- Export includes a manifest and checksums.
- Import validates the manifest and checksums.
- Imported assets get previews and metadata.
- Failures are reported per asset, not as a truncated stream.
- Imported assets can be backed up by the destination library's backup settings.

---

## Workflow 2 — Continuous backup

### v1: current per-library SFTP mirror

Already implemented:

- Per-library configuration in the UI.
- Originals are copied asynchronously after upload/replace.
- Deletes remove the remote copy.
- A periodic reconciliation queues missing/failed assets.
- Credentials are encrypted; passwords are never returned by the API.

Limitations:

- The remote key is mutable: `libraries/{library}/assets/{assetID}{ext}`.
- Replacing content overwrites the same remote key.
- Deleting an asset removes the remote blob immediately.
- There is no DB replication.
- Therefore, this is **not** point-in-time recovery. It is a live mirror.

### v2: versioned stream + DB snapshots (point-in-time recovery)

Goal: lightweight DB snapshots + an append-only storage stream, so any DB
snapshot can be paired with the continuous storage state without zipping the
entire storage each time.

The core invariant:

> A DB snapshot at time T is restorable when every blob version referenced by
> that snapshot still exists at the backup destination.

#### 1. Add content identity to the data model

Add to `assets` (or create `asset_versions`):

- `content_version` (monotonic integer) or `content_sha256`
- optionally `content_updated_at`

Update it atomically on upload and replace. The DB row must identify the exact
blob version used at any point in time.

If you want version history/undo in the UI, add:

```text
asset_versions
  id
  asset_id
  storage_key
  sha256
  byte_size
  created_at
  deleted_at
```

and point `assets.current_version_id` at the active row.

Metadata/attribute edits do **not** need versions; DB snapshots capture those
changes.

#### 2. Make remote backup keys versioned

Change `backupKey()` in `service/backup_service.go` from:

```text
libraries/{libraryID}/assets/{assetID}{ext}
```

to something like:

```text
libraries/{libraryID}/assets/{assetID}/{contentVersion}{ext}
```

or content-addressed:

```text
blobs/{sha256Prefix}/{sha256}{ext}
```

Content-addressed keys give deduplication for free.

#### 3. Change delete and replace semantics

- Replace: write a new remote version; never overwrite the old one.
- Delete: do not remove the remote blob immediately. Record a tombstone or
  simply rely on retention + GC.
- Add a GC job that deletes blobs older than the retention window that are no
  longer referenced by the current DB.
- Keep deleted-asset blobs for at least the DB snapshot retention window.

#### 4. Add lightweight DB snapshots

Add a `db_snapshots` table and a scheduler:

```text
db_snapshots
  id
  created_at
  path / destination
  schema_version
  sha256
  size
  status
  last_error
```

Snapshot with SQLite's consistent mechanisms:

- `VACUUM INTO 'snapshot.db'`, or
- the SQLite online backup API.

Do not copy `godam.db` raw while WAL mode is active.

Retain N snapshots for R days. Verify with `PRAGMA integrity_check`.

Continuous DB replication (WAL shipping) can be added later with a mature tool
such as Litestream; periodic snapshots are simpler and sufficient initially.

#### 5. Enforce backup consistency before snapshotting

A DB snapshot must not outrun the async blob copy. Either:

- flush/wait for the backup queue to drain before taking a snapshot, or
- store a backup high-water mark in the DB and only mark the snapshot
  restorable once the destination has replicated through that mark.

Restore must verify that every referenced blob exists. Missing blobs mean the
snapshot is not restorable.

#### 6. Restore procedure

1. Restore the DB snapshot into a temporary database.
2. Validate schema version and `PRAGMA integrity_check`.
3. For each asset version referenced by the snapshot, fetch the blob from the
   continuous destination.
4. Verify size/checksum.
5. Rebuild the local storage directory, or restore directly to a new app data
   directory.
6. Regenerate previews (or restore them if included).
7. Re-enter backup credentials if `backup.key` was not included.

Provide this as an offline CLI first, e.g.:

```text
godam restore --db snapshot.db --storage sftp://...
```

Importing a full database through a live HTTP request is risky because it
replaces state under active sessions.

#### 7. Gate the feature correctly

- DB snapshots alone are not PIT-complete.
- Continuous storage alone is not PIT-complete (no catalog history).
- Point-in-time recovery requires both.
- If continuous storage is enabled later, PIT starts from that moment; earlier
  DB snapshots cannot be paired with history that does not exist.
- In the UI, show a warning when only one half is configured.

### Acceptance criteria for v2

- Replaced content creates a new remote version; old versions remain.
- Deleted assets leave remote blobs until retention/GC.
- DB snapshots are consistent and lightweight.
- A DB snapshot can be restored using only the DB snapshot + continuous
  destination (no full storage zip).
- Missing blobs are detected before restore is declared successful.
- Retention prevents unbounded growth and still covers the oldest DB snapshot.

---

## Workflow 3 — Full snapshot (DB + storage + manifest)

### Use cases

- Move an entire godam instance to another server.
- Point-in-time recovery when continuous versioned storage was not enabled.
- Disaster recovery where a single self-contained archive is preferred.

### Archive layout

```text
godam-snapshot.zip
  manifest.json
  godam.db
  storage/
    uploads/...
    previews/...        (optional)
```

`manifest.json`:

```json
{
  "format": "godam.full-snapshot",
  "format_version": 1,
  "created_at": "...",
  "godam_version": "...",
  "schema_version": 15,
  "counts": {
    "libraries": 3,
    "assets": 1234,
    "bytes": 123456789
  },
  "includes_previews": false,
  "checksums": {
    "godam.db": "sha256:..."
  }
}
```

### Export

Implementation steps:

1. Enter a short maintenance mode / quiesce writes, or accept a documented
   consistency window.
2. Take a consistent DB snapshot (`VACUUM INTO` or backup API).
3. Copy `storage/uploads/` (and optionally `storage/previews/`).
4. Generate `manifest.json` with versions, counts, and checksums.
5. Stream a ZIP (ZIP64) or `tar.zst` to the client or a configured destination.
6. Exit maintenance mode.

For large instances:

- Prefer a background export job with progress and a download link.
- Or write directly to a destination (local path/SFTP) instead of streaming
  through the browser.

### Restore

1. Stop the application.
2. Validate the manifest and checksums.
3. Replace `godam.db` and `storage/`.
4. Do not restore a DB with a newer schema version than the app supports.
5. Start the app; migrations run forward.
6. Re-enter backup credentials if `backup.key` was not included.

Previews are derived: excluding them keeps snapshots smaller; including them
makes restores instant.

### Acceptance criteria

- Snapshot contains DB + storage + manifest.
- DB snapshot is consistent (not a raw copy of a live WAL database).
- Restore works offline and preserves asset IDs/storage keys.
- Version compatibility is checked before replacing state.

---

## Workflow comparison

| Workflow | Moves | Point-in-time? | Typical use |
| --- | --- | --- | --- |
| Selective ZIP + manifest | Assets and metadata | No | Move assets between libraries/instances |
| Continuous v1 | Originals (latest) | No | Live mirror, quick DR |
| Continuous v2 | Versioned originals + DB snapshots | Yes | Low-cost PIT recovery |
| Full snapshot | DB + storage + manifest | Yes, at snapshot time | Migration, complete recovery |

## Recommended implementation order

1. **Selective export/import with manifest**
   - Add manifest generation to the ZIP endpoint.
   - Add ZIP import with validation and checksum verification.
2. **Full snapshot export/restore**
   - Add consistent DB snapshot + storage archive.
   - Add an offline restore CLI.
3. **Continuous v2 (PIT)**
   - Add content version/hash to the data model.
   - Version remote backup keys.
   - Change delete/replace to retained versions + GC.
   - Add DB snapshots and queue-drain/high-water-mark consistency.
   - Add restore tooling and UI warnings for partial configuration.

## Non-goals for the first pass

- S3 as a primary storage driver.
- Multiple backup destinations per library.
- Retention policies for individual assets.
- Version history/undo UI (unless `asset_versions` is introduced).
- Zero-downtime live full-state import.
