package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"audiod/internal/config"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// EmbedMigrations returns the embedded migration files for use in tests.
func EmbedMigrations() embed.FS {
	return embedMigrations
}

// Open opens a database connection and runs migrations
func Open() (*sql.DB, error) {
	dataDir := config.GetDataDir()

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := fmt.Sprintf("%s/audiod.db", dataDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite is a single-writer DB; the modernc.org driver's PRAGMAs (notably
	// busy_timeout) are also per-connection. With Go's default pool, only the
	// first connection inherits the PRAGMAs we run below — every additional
	// connection the pool spins up gets fresh defaults (busy_timeout=0),
	// which is why a concurrent scanner worker + a handler write surfaced as
	// instant SQLITE_BUSY → HTTP 500 in CI. Pinning to a single connection
	// lets database/sql serialize callers in Go (each request waits its turn)
	// instead of racing through multiple un-tuned SQLite connections.
	// The existing test memory ("sqlite :memory: needs single-conn pool in
	// tests") flags the same gotcha — production has the same shape.
	db.SetMaxOpenConns(1)

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Wait up to 5s for the write lock instead of returning SQLITE_BUSY
	// immediately. Lets the server and CLI cooperate on the same DB (CLI
	// opens its own connection from a separate process).
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	// Run migrations
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations complete")
	return db, nil
}
