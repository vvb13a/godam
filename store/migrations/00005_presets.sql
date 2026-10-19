-- +goose Up
-- Named, reusable imgproxy processing pipelines. name is the URL-safe key used
-- in conversion URLs; parameters is the JSON option list.
CREATE TABLE presets (
    id TEXT PRIMARY KEY,
    added_by TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL,
    name TEXT NOT NULL UNIQUE,
    parameters TEXT NOT NULL DEFAULT '{"format":"webp","options":[]}',
    created_at DATETIME NOT NULL,
    modified_at DATETIME NOT NULL
);

CREATE INDEX idx_presets_added_by ON presets(added_by);

-- +goose Down
DROP TABLE presets;
