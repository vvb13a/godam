-- +goose Up
ALTER TABLE attributes_metadata ADD COLUMN searchable INTEGER NOT NULL DEFAULT 0;
ALTER TABLE attributes_metadata ADD COLUMN multi INTEGER NOT NULL DEFAULT 1;
ALTER TABLE attributes_metadata ADD COLUMN include_unassigned INTEGER NOT NULL DEFAULT 0;

-- Attribute values are indexed unconditionally; the search query only matches
-- keys whose metadata is marked searchable. This keeps toggling cheap.
CREATE VIRTUAL TABLE asset_attribute_search USING fts5(
    asset_id UNINDEXED,
    key UNINDEXED,
    value,
    tokenize='trigram'
);

INSERT INTO asset_attribute_search(asset_id, key, value)
SELECT asset_id, key, value FROM asset_attributes;

-- +goose StatementBegin
CREATE TRIGGER asset_attributes_search_insert
AFTER INSERT ON asset_attributes BEGIN
    INSERT INTO asset_attribute_search(asset_id, key, value)
    VALUES (new.asset_id, new.key, new.value);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER asset_attributes_search_delete
AFTER DELETE ON asset_attributes BEGIN
    DELETE FROM asset_attribute_search
    WHERE asset_id = old.asset_id AND key = old.key AND value = old.value;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER asset_attributes_search_update
AFTER UPDATE ON asset_attributes BEGIN
    DELETE FROM asset_attribute_search
    WHERE asset_id = old.asset_id AND key = old.key AND value = old.value;
    INSERT INTO asset_attribute_search(asset_id, key, value)
    VALUES (new.asset_id, new.key, new.value);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS asset_attributes_search_update;
DROP TRIGGER IF EXISTS asset_attributes_search_delete;
DROP TRIGGER IF EXISTS asset_attributes_search_insert;
DROP TABLE IF EXISTS asset_attribute_search;
ALTER TABLE attributes_metadata DROP COLUMN include_unassigned;
ALTER TABLE attributes_metadata DROP COLUMN multi;
ALTER TABLE attributes_metadata DROP COLUMN searchable;
