-- +goose Up
-- Track identity survives file moves (docs/adr/0002). The scanner relinks a
-- new path to a vanished track by recording id, content hash or tags; these
-- back those lookups.
ALTER TABLE track ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_track_content_hash ON track (library_id, content_hash);
CREATE INDEX idx_track_musicbrainz_id ON track (library_id, musicbrainz_id);
CREATE INDEX idx_album_musicbrainz_id ON album (library_id, musicbrainz_id);

-- +goose Down
DROP INDEX IF EXISTS idx_album_musicbrainz_id;
DROP INDEX IF EXISTS idx_track_musicbrainz_id;
DROP INDEX IF EXISTS idx_track_content_hash;
ALTER TABLE track DROP COLUMN content_hash;
