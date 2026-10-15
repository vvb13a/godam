-- +goose Up
-- Seed the built-in Uncategorized/Untagged filter presets for every library
-- that existed before they shipped. New libraries get them at creation time.
-- They are ordinary smart collections, so users may rename, recolour or delete
-- them freely.

INSERT INTO smart_collections (id, library_id, name, color, sort, filter, created_at, updated_at)
SELECT lower(hex(randomblob(16))), l.id, 'Uncategorized', '8b5cf6', 0, '{"collection":"uncategorized"}', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM libraries l
WHERE NOT EXISTS (
    SELECT 1 FROM smart_collections sc
    WHERE sc.library_id = l.id AND sc.name = 'Uncategorized'
);

INSERT INTO smart_collections (id, library_id, name, color, sort, filter, created_at, updated_at)
SELECT lower(hex(randomblob(16))), l.id, 'Untagged', '8b5cf6', 1, '{"tag":"untagged"}', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM libraries l
WHERE NOT EXISTS (
    SELECT 1 FROM smart_collections sc
    WHERE sc.library_id = l.id AND sc.name = 'Untagged'
);

-- +goose Down
-- Remove the seeded presets only while they still carry their default filter,
-- so a user-renamed or re-filtered row is left untouched.
DELETE FROM smart_collections
WHERE (name = 'Uncategorized' AND filter = '{"collection":"uncategorized"}')
   OR (name = 'Untagged' AND filter = '{"tag":"untagged"}');
