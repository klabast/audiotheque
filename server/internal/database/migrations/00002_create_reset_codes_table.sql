-- +goose Up
CREATE TABLE IF NOT EXISTS reset_code
(
    code       TEXT PRIMARY KEY,
    user_id    INTEGER  NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES user (id) ON DELETE CASCADE
);

CREATE INDEX idx_reset_code_user_id ON reset_code (user_id);
CREATE INDEX idx_reset_code_expires_at ON reset_code (expires_at);

-- +goose Down
DROP TABLE IF EXISTS reset_code;
