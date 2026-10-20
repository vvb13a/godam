-- +goose Up
-- Built-in Uncategorized/Untagged views are now regular sidebar links.
DELETE FROM smart_collections
WHERE (name = 'Uncategorized' AND filter = '{"collection":"uncategorized"}')
   OR (name = 'Untagged' AND filter = '{"tag":"untagged"}');

-- +goose Down
-- Deleted built-in presets are not restored.
