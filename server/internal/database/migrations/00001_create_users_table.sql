-- +goose Up
CREATE TABLE IF NOT EXISTS user
(
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT     NOT NULL UNIQUE,
    password_hash TEXT     NOT NULL,
    is_admin      BOOLEAN  NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_username ON user (username);

-- +goose Down
DROP TABLE IF EXISTS user;
