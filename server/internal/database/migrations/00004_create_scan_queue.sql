-- +goose Up
-- +goose StatementBegin
CREATE TABLE scan_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL REFERENCES library(id) ON DELETE CASCADE,
    requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,  -- heartbeat for orphan detection
    status TEXT DEFAULT 'pending',  -- pending, running

    -- Progress tracking
    total_files INTEGER DEFAULT 0,
    processed_files INTEGER DEFAULT 0,
    tracks_added INTEGER DEFAULT 0,
    tracks_updated INTEGER DEFAULT 0,
    errors INTEGER DEFAULT 0,
    current_file TEXT DEFAULT ''
);

CREATE INDEX idx_scan_queue_status ON scan_queue(status);
CREATE INDEX idx_scan_queue_library ON scan_queue(library_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_scan_queue_library;
DROP INDEX IF EXISTS idx_scan_queue_status;
DROP TABLE IF EXISTS scan_queue;
-- +goose StatementEnd
