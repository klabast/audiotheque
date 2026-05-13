-- +goose Up
-- One playback session per user (Spotify model). Arrays/maps stored as JSON
-- because SQLite handles them transparently and they only need to round-trip,
-- not be queried over.
CREATE TABLE playback_session (
    user_id INTEGER PRIMARY KEY REFERENCES user(id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'stopped',
    current_track_id INTEGER,
    current_position INTEGER NOT NULL DEFAULT 0,
    source_type TEXT NOT NULL DEFAULT '',
    source_id INTEGER NOT NULL DEFAULT 0,
    source_remaining TEXT NOT NULL DEFAULT '[]',
    queue TEXT NOT NULL DEFAULT '[]',
    history TEXT NOT NULL DEFAULT '[]',
    device_id TEXT NOT NULL DEFAULT '',
    device_volumes TEXT NOT NULL DEFAULT '{}',
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS playback_session;
