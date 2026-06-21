-- +goose Up
-- Full-text search over the library using SQLite FTS5. The old search used
-- `title LIKE '%q%'` per entity, which can't match by artist, ignores accents,
-- breaks on multi-word queries, and has no relevance ranking. These contentless
-- FTS5 tables index title/name plus the denormalized artist name, genre and
-- year so a query like "pink moon" finds Dark Side of the Moon by artist, and
-- "bjork" finds Björk. library_id is stored UNINDEXED so searches can be scoped
-- to one library without joining back to the base table.
--
-- The tables are kept in sync by triggers below. artist.name is immutable
-- (get-or-create dedup key) so renames never happen; track metadata is rewritten
-- wholesale on rescan, hence the track update trigger refreshes its row.

CREATE VIRTUAL TABLE album_fts USING fts5(
    title, artist, genre, year,
    library_id UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE VIRTUAL TABLE track_fts USING fts5(
    title, artist, genre, year,
    library_id UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE VIRTUAL TABLE artist_fts USING fts5(
    name,
    library_id UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2'
);

-- +goose StatementBegin
CREATE TRIGGER album_ai AFTER INSERT ON album BEGIN
    INSERT INTO album_fts(rowid, title, artist, genre, year, library_id)
    VALUES (new.id, new.title,
            COALESCE((SELECT name FROM artist WHERE id = new.artist_id), ''),
            COALESCE(new.genre, ''),
            COALESCE(substr(new.release_date, 1, 4), ''),
            new.library_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER album_ad AFTER DELETE ON album BEGIN
    DELETE FROM album_fts WHERE rowid = old.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER album_au AFTER UPDATE ON album BEGIN
    DELETE FROM album_fts WHERE rowid = old.id;
    INSERT INTO album_fts(rowid, title, artist, genre, year, library_id)
    VALUES (new.id, new.title,
            COALESCE((SELECT name FROM artist WHERE id = new.artist_id), ''),
            COALESCE(new.genre, ''),
            COALESCE(substr(new.release_date, 1, 4), ''),
            new.library_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER track_ai AFTER INSERT ON track BEGIN
    INSERT INTO track_fts(rowid, title, artist, genre, year, library_id)
    VALUES (new.id, new.title,
            COALESCE((SELECT name FROM artist WHERE id = new.artist_id), ''),
            COALESCE(new.genre, ''),
            CASE WHEN new.year > 0 THEN CAST(new.year AS TEXT) ELSE '' END,
            new.library_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER track_ad AFTER DELETE ON track BEGIN
    DELETE FROM track_fts WHERE rowid = old.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER track_au AFTER UPDATE ON track BEGIN
    DELETE FROM track_fts WHERE rowid = old.id;
    INSERT INTO track_fts(rowid, title, artist, genre, year, library_id)
    VALUES (new.id, new.title,
            COALESCE((SELECT name FROM artist WHERE id = new.artist_id), ''),
            COALESCE(new.genre, ''),
            CASE WHEN new.year > 0 THEN CAST(new.year AS TEXT) ELSE '' END,
            new.library_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER artist_ai AFTER INSERT ON artist BEGIN
    INSERT INTO artist_fts(rowid, name, library_id)
    VALUES (new.id, new.name, new.library_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER artist_ad AFTER DELETE ON artist BEGIN
    DELETE FROM artist_fts WHERE rowid = old.id;
END;
-- +goose StatementEnd

-- SQLite foreign-key cascade deletes do NOT fire row triggers (recursive_triggers
-- is off by default), so deleting a library would orphan its FTS rows. Clear them
-- explicitly before the cascade removes the base rows.
-- +goose StatementBegin
CREATE TRIGGER library_bd_fts BEFORE DELETE ON library BEGIN
    DELETE FROM album_fts WHERE rowid IN (SELECT id FROM album WHERE library_id = old.id);
    DELETE FROM track_fts WHERE rowid IN (SELECT id FROM track WHERE library_id = old.id);
    DELETE FROM artist_fts WHERE rowid IN (SELECT id FROM artist WHERE library_id = old.id);
END;
-- +goose StatementEnd

-- Backfill existing rows so libraries scanned before this migration are searchable
-- without a rescan.
INSERT INTO album_fts(rowid, title, artist, genre, year, library_id)
SELECT a.id, a.title, COALESCE(ar.name, ''), COALESCE(a.genre, ''),
       COALESCE(substr(a.release_date, 1, 4), ''), a.library_id
FROM album a
LEFT JOIN artist ar ON ar.id = a.artist_id;

INSERT INTO track_fts(rowid, title, artist, genre, year, library_id)
SELECT t.id, t.title, COALESCE(ar.name, ''), COALESCE(t.genre, ''),
       CASE WHEN t.year > 0 THEN CAST(t.year AS TEXT) ELSE '' END, t.library_id
FROM track t
LEFT JOIN artist ar ON ar.id = t.artist_id;

INSERT INTO artist_fts(rowid, name, library_id)
SELECT id, name, library_id FROM artist;

-- +goose Down
DROP TRIGGER IF EXISTS library_bd_fts;
DROP TRIGGER IF EXISTS artist_ad;
DROP TRIGGER IF EXISTS artist_ai;
DROP TRIGGER IF EXISTS track_au;
DROP TRIGGER IF EXISTS track_ad;
DROP TRIGGER IF EXISTS track_ai;
DROP TRIGGER IF EXISTS album_au;
DROP TRIGGER IF EXISTS album_ad;
DROP TRIGGER IF EXISTS album_ai;
DROP TABLE IF EXISTS artist_fts;
DROP TABLE IF EXISTS track_fts;
DROP TABLE IF EXISTS album_fts;
