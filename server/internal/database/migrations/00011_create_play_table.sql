-- +goose Up
-- One row per track played: how long it ran before playback moved on. Whether
-- that was a listen or a skip is judged when reading, against the track's
-- duration, so the rule can change without rewriting history.
CREATE TABLE play
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id        INTEGER  NOT NULL REFERENCES user (id) ON DELETE CASCADE,
    track_id       INTEGER  NOT NULL REFERENCES track (id) ON DELETE CASCADE,
    played_seconds INTEGER  NOT NULL,
    ended_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_play_user_ended_at ON play (user_id, ended_at);
CREATE INDEX idx_play_user_track ON play (user_id, track_id);
CREATE INDEX idx_play_track ON play (track_id);

-- +goose Down
DROP TABLE IF EXISTS play;
