-- +goose Up
-- FTS5 trigram index for substring search over asset text. External content
-- keeps the original text in assets; triggers keep the index in sync.
CREATE VIRTUAL TABLE asset_search USING fts5(
    filename,
    title,
    note,
    content='assets',
    content_rowid='rowid',
    tokenize='trigram'
);

INSERT INTO asset_search(asset_search) VALUES ('rebuild');

-- +goose StatementBegin
CREATE TRIGGER assets_search_insert AFTER INSERT ON assets BEGIN
    INSERT INTO asset_search(rowid, filename, title, note)
    VALUES (new.rowid, new.filename, new.title, new.note);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER assets_search_delete AFTER DELETE ON assets BEGIN
    INSERT INTO asset_search(asset_search, rowid, filename, title, note)
    VALUES ('delete', old.rowid, old.filename, old.title, old.note);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER assets_search_update
AFTER UPDATE OF filename, title, note ON assets BEGIN
    INSERT INTO asset_search(asset_search, rowid, filename, title, note)
    VALUES ('delete', old.rowid, old.filename, old.title, old.note);
    INSERT INTO asset_search(rowid, filename, title, note)
    VALUES (new.rowid, new.filename, new.title, new.note);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS assets_search_update;
DROP TRIGGER IF EXISTS assets_search_delete;
DROP TRIGGER IF EXISTS assets_search_insert;
DROP TABLE IF EXISTS asset_search;
