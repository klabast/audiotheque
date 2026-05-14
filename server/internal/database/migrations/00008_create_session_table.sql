-- +goose Up
-- DB-backed browser sessions. Replaces stateless JWT for the auth cookie.
-- Stream tokens (MPD audio fetch) remain JWTs — different concern.
--
-- id is an opaque random 32-byte value (URL-safe base64, ~43 chars).
-- expires_at is sliding-renewed when the user is active. remember_me drives
-- the renewal window (30d off, 90d on). user_agent + last_ip are recorded
-- for the Active Devices UI in a follow-up slice.
CREATE TABLE IF NOT EXISTS session
(
    id           TEXT PRIMARY KEY,
    user_id      INTEGER  NOT NULL REFERENCES user (id) ON DELETE CASCADE,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   DATETIME NOT NULL,
    remember_me  BOOLEAN  NOT NULL DEFAULT 0,
    user_agent   TEXT     NOT NULL DEFAULT '',
    last_ip      TEXT     NOT NULL DEFAULT ''
);

CREATE INDEX idx_session_user_id ON session (user_id);
CREATE INDEX idx_session_expires_at ON session (expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_session_expires_at;
DROP INDEX IF EXISTS idx_session_user_id;
DROP TABLE IF EXISTS session;
