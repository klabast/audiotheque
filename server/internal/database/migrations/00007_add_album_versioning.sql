-- +goose Up
-- Multi-version albums: when the same release exists in multiple folders
-- (e.g. standard + 24-bit hi-res rip), they must be stored as separate album
-- rows or tracks from different folders collide into one album. The fix from
-- v1: dedup on (library_id, artist_id, title, folder_path) instead of just
-- (library_id, artist_id, title). release_type is a derived label for UI use.
ALTER TABLE album ADD COLUMN folder_path TEXT NOT NULL DEFAULT '';
ALTER TABLE album ADD COLUMN release_type TEXT NOT NULL DEFAULT 'original';

CREATE INDEX idx_album_folder_path ON album (folder_path);

-- +goose Down
DROP INDEX IF EXISTS idx_album_folder_path;
ALTER TABLE album DROP COLUMN release_type;
ALTER TABLE album DROP COLUMN folder_path;
