-- +goose Up
-- Libraries become shared: a library can have many members and a user can
-- belong to many libraries. Membership replaces the single-owner column.

CREATE TABLE library_users (
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (library_id, user_id)
);

CREATE INDEX idx_library_users_user ON library_users(user_id);

-- Adopt the previous single owner as the first member.
INSERT INTO library_users (library_id, user_id)
SELECT id, user_id FROM libraries WHERE user_id IS NOT NULL AND user_id <> '';

DROP INDEX IF EXISTS idx_libraries_user;
ALTER TABLE libraries DROP COLUMN user_id;

-- +goose Down
ALTER TABLE libraries ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE CASCADE;
UPDATE libraries SET user_id = (
    SELECT user_id FROM library_users WHERE library_id = libraries.id LIMIT 1
);
DROP INDEX IF EXISTS idx_library_users_user;
DROP TABLE IF EXISTS library_users;
