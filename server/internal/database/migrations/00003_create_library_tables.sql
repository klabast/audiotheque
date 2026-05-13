-- +goose Up

-- =============================================================================
-- LIBRARY TABLES
-- =============================================================================

-- Library table: first-class entities that users can access
CREATE TABLE IF NOT EXISTS library
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_library_name ON library (name);

-- Library path: libraries can have multiple filesystem paths
CREATE TABLE IF NOT EXISTS library_path
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER  NOT NULL,
    path       TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES library (id) ON DELETE CASCADE,
    UNIQUE (library_id, path)
);

CREATE INDEX idx_library_path_library_id ON library_path (library_id);
CREATE INDEX idx_library_path_path ON library_path (path);

-- Library access: many-to-many relationship between users and libraries
CREATE TABLE IF NOT EXISTS library_access
(
    user_id    INTEGER NOT NULL,
    library_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, library_id),
    FOREIGN KEY (user_id) REFERENCES user (id) ON DELETE CASCADE,
    FOREIGN KEY (library_id) REFERENCES library (id) ON DELETE CASCADE
);

CREATE INDEX idx_library_access_user_id ON library_access (user_id);
CREATE INDEX idx_library_access_library_id ON library_access (library_id);

-- =============================================================================
-- ARTIST TABLE
-- =============================================================================
-- Artists are library-scoped to handle same-name artists across different libraries

CREATE TABLE IF NOT EXISTS artist
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id     INTEGER  NOT NULL,
    name           TEXT     NOT NULL,
    sort_name      TEXT,
    musicbrainz_id TEXT,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES library (id) ON DELETE CASCADE,
    UNIQUE (library_id, name)
);

CREATE INDEX idx_artist_library_id ON artist (library_id);
CREATE INDEX idx_artist_name ON artist (name COLLATE NOCASE);

-- =============================================================================
-- ALBUM TABLE
-- =============================================================================

CREATE TABLE IF NOT EXISTS album
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id     INTEGER  NOT NULL,
    artist_id      INTEGER,                          -- NULL for compilations
    title          TEXT     NOT NULL,
    sort_title     TEXT,
    musicbrainz_id TEXT,
    release_date   TEXT,                             -- YYYY or YYYY-MM-DD
    genre          TEXT,
    total_tracks   INTEGER,
    total_discs    INTEGER  DEFAULT 1,
    cover_art_path TEXT,
    is_compilation INTEGER  NOT NULL DEFAULT 0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES library (id) ON DELETE CASCADE,
    FOREIGN KEY (artist_id) REFERENCES artist (id) ON DELETE SET NULL
);

CREATE INDEX idx_album_library_id ON album (library_id);
CREATE INDEX idx_album_artist_id ON album (artist_id);
CREATE INDEX idx_album_title ON album (title COLLATE NOCASE);

-- =============================================================================
-- TRACK TABLE
-- =============================================================================

CREATE TABLE IF NOT EXISTS track
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id     INTEGER  NOT NULL,
    album_id       INTEGER,
    artist_id      INTEGER,                          -- Primary track artist

    -- File information (for incremental scanning)
    file_path      TEXT     NOT NULL,
    file_name      TEXT     NOT NULL,
    file_size      INTEGER  NOT NULL,
    file_modified  DATETIME NOT NULL,

    -- Track metadata
    title          TEXT     NOT NULL,
    sort_title     TEXT,
    track_number   INTEGER,
    disc_number    INTEGER  DEFAULT 1,
    duration       INTEGER  NOT NULL DEFAULT 0,      -- milliseconds
    year           INTEGER,
    genre          TEXT,
    musicbrainz_id TEXT,

    -- Audio properties
    codec          TEXT     NOT NULL,                -- flac, mp3, aac, etc.
    bitrate        INTEGER,                          -- kbps (lossy only)
    sample_rate    INTEGER  NOT NULL,                -- Hz
    bit_depth      INTEGER,                          -- bits (16, 24, 32)
    channels       INTEGER  NOT NULL DEFAULT 2,
    is_lossless    INTEGER  NOT NULL DEFAULT 0,
    is_hires       INTEGER  NOT NULL DEFAULT 0,      -- bit_depth >= 24 OR sample_rate >= 48000

    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (library_id) REFERENCES library (id) ON DELETE CASCADE,
    FOREIGN KEY (album_id) REFERENCES album (id) ON DELETE SET NULL,
    FOREIGN KEY (artist_id) REFERENCES artist (id) ON DELETE SET NULL,
    UNIQUE (library_id, file_path)
);

CREATE INDEX idx_track_library_id ON track (library_id);
CREATE INDEX idx_track_album_id ON track (album_id);
CREATE INDEX idx_track_artist_id ON track (artist_id);
CREATE INDEX idx_track_file_path ON track (file_path);
CREATE INDEX idx_track_title ON track (title COLLATE NOCASE);

-- =============================================================================
-- TRACK-ARTIST RELATIONSHIP (Many-to-Many)
-- =============================================================================
-- For tracks with multiple artists (features, collaborations)
-- Primary track artist is in track.artist_id, additional ones here

CREATE TABLE IF NOT EXISTS track_artist
(
    track_id  INTEGER NOT NULL,
    artist_id INTEGER NOT NULL,
    role      TEXT    NOT NULL DEFAULT 'primary',   -- primary, featured, remixer
    position  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (track_id, artist_id, role),
    FOREIGN KEY (track_id) REFERENCES track (id) ON DELETE CASCADE,
    FOREIGN KEY (artist_id) REFERENCES artist (id) ON DELETE CASCADE
);

CREATE INDEX idx_track_artist_track_id ON track_artist (track_id);
CREATE INDEX idx_track_artist_artist_id ON track_artist (artist_id);

-- +goose Down
DROP TABLE IF EXISTS track_artist;
DROP TABLE IF EXISTS track;
DROP TABLE IF EXISTS album;
DROP TABLE IF EXISTS artist;
DROP TABLE IF EXISTS library_access;
DROP TABLE IF EXISTS library_path;
DROP TABLE IF EXISTS library;
