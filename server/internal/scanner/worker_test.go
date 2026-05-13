package scanner

import (
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	"audiod/internal/database"
	"audiod/internal/library"
	"audiod/internal/websocket"
)

// mockBroadcaster records every message it's asked to send so tests can
// assert what the worker emitted.
type mockBroadcaster struct {
	mu       sync.Mutex
	messages []websocket.Message
}

func (m *mockBroadcaster) Broadcast(msg websocket.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockBroadcaster) snapshot() []websocket.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]websocket.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

// setupTestWorker wires up an in-memory SQLite DB with all migrations applied,
// seeds a single library row, and returns a Worker bound to a real Repository
// (no WebSocket hub — passing nil disables broadcasts in worker.broadcastProgress).
func setupTestWorker(t *testing.T) (*Worker, library.Repository, *sql.DB, int64) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// modernc.org/sqlite gives each pool connection its own private :memory:
	// database. The worker spawns a goroutine that queries concurrently with
	// the test, which would land on a separate (empty) connection. Pin to one
	// connection so migrations are visible everywhere.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable fk: %v", err)
	}

	goose.SetBaseFS(database.EmbedMigrations())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	res, err := db.Exec("INSERT INTO library (name) VALUES ('test')")
	if err != nil {
		t.Fatalf("seed library: %v", err)
	}
	libID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("library id: %v", err)
	}

	repo := library.NewRepository(db)
	w := NewWorker(repo, nil)
	return w, repo, db, libID
}

// insertRunningScanJob inserts a scan_queue row with status='running' and the
// given heartbeat timestamp (used for both started_at and updated_at).
func insertRunningScanJob(t *testing.T, db *sql.DB, libID int64, heartbeat time.Time) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO scan_queue (library_id, status, started_at, updated_at)
		 VALUES (?, 'running', ?, ?)`,
		libID, heartbeat.UTC(), heartbeat.UTC(),
	)
	if err != nil {
		t.Fatalf("insert running scan_queue row: %v", err)
	}
}

// TestWorker_Start_ResetsAllRunningJobsRegardlessOfHeartbeat is the regression
// test for the 409 Conflict bug after container restart.
//
// Bug: on boot, the worker only reset rows where updated_at was older than
// OrphanTimeout (2 min). If the container was restarted right after a scan
// heartbeat, the stale 'running' row was left untouched — and StartScan then
// returned ErrScanAlreadyInProgress forever (HTTP 409) until either another
// restart >2 min later, or manual SQL.
//
// Behavioural fix: at worker boot, every row with status='running' must be
// reset to 'pending', regardless of heartbeat freshness, because by definition
// no worker process is running yet at boot time.
//
// We exercise the method that Worker.Start() invokes synchronously, so the
// assertion is not racy with the worker's background goroutine.
func TestWorker_Start_ResetsAllRunningJobsRegardlessOfHeartbeat(t *testing.T) {
	w, repo, db, libID := setupTestWorker(t)

	// Fresh heartbeat — well within OrphanTimeout, so the old timeout-based
	// reset would have left this row 'running'.
	insertRunningScanJob(t, db, libID, time.Now())

	w.resetAllRunningJobs()

	job, err := repo.GetScanJobByLibrary(libID)
	if err != nil {
		t.Fatalf("GetScanJobByLibrary: %v", err)
	}
	if job == nil {
		t.Fatal("expected scan_queue row to still exist; it was deleted")
	}
	if job.Status != "pending" {
		t.Fatalf("status: want pending, got %q", job.Status)
	}
	if job.StartedAt != nil {
		t.Fatalf("started_at: want NULL, got %v", job.StartedAt)
	}
}

// TestWorker_PeriodicallyResetsStaleOrphans verifies the secondary defense:
// while the worker process is alive, a 'running' row whose heartbeat has
// gone stale (older than OrphanTimeout) is reset back to 'pending' by the
// periodic maintenance pass. This protects against an in-process scan that
// stops heartbeating (e.g. processJob panics) without a full restart.
func TestWorker_PeriodicallyResetsStaleOrphans(t *testing.T) {
	w, repo, db, libID := setupTestWorker(t)

	// Disable the regular pending-pickup ticker so it can't race the
	// assertion by picking up the row once maintenance has reset it.
	w.interval = time.Hour
	// Maintenance ticks fast enough that we don't have to sit in the test.
	w.maintenanceInterval = 50 * time.Millisecond

	w.Start()
	defer w.Stop()

	// Insert AFTER Start so the boot-time reset doesn't trivially clear
	// the row; we want to exercise the periodic path specifically.
	insertRunningScanJob(t, db, libID, time.Now().Add(-5*time.Minute))

	// Allow several maintenance ticks.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := repo.GetScanJobByLibrary(libID)
		if err != nil {
			t.Fatalf("GetScanJobByLibrary: %v", err)
		}
		if job != nil && job.Status == "pending" {
			return // success
		}
		time.Sleep(25 * time.Millisecond)
	}

	job, _ := repo.GetScanJobByLibrary(libID)
	if job == nil {
		t.Fatal("scan_queue row was deleted; expected it to be reset to 'pending'")
	}
	t.Fatalf("periodic maintenance did not reset stale 'running' row; final status = %q", job.Status)
}

// TestWorker_broadcastLibraryUpdated_EmitsMessage is the unit test for the new
// 'library-updated' event. The UI subscribes to this to refetch albums live
// during a scan (bugs 3 + 4 — combined scan-progress global-store refactor).
func TestWorker_broadcastLibraryUpdated_EmitsMessage(t *testing.T) {
	w, _, _, _ := setupTestWorker(t)
	mb := &mockBroadcaster{}
	w.broadcaster = mb

	w.broadcastLibraryUpdated(42)

	msgs := mb.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(msgs))
	}
	if msgs[0].Type != "library-updated" {
		t.Fatalf("type: want %q, got %q", "library-updated", msgs[0].Type)
	}
	// Payload must carry the affected libraryId so the UI knows what to refetch.
	data, ok := msgs[0].Data.(map[string]int64)
	if !ok {
		t.Fatalf("data: want map[string]int64, got %T", msgs[0].Data)
	}
	if data["libraryId"] != 42 {
		t.Fatalf("libraryId: want 42, got %d", data["libraryId"])
	}
}

// TestWorker_maybeBroadcastLibraryUpdated_Throttles ensures per-track emissions
// during a big scan don't flood the WebSocket. A single library should only
// see one event per libraryUpdatedThrottle window.
func TestWorker_maybeBroadcastLibraryUpdated_Throttles(t *testing.T) {
	w, _, _, _ := setupTestWorker(t)
	mb := &mockBroadcaster{}
	w.broadcaster = mb
	// Short window so the test stays fast.
	w.libraryUpdatedThrottle = 100 * time.Millisecond

	// First call goes through.
	w.maybeBroadcastLibraryUpdated(7)
	// Immediate second call is suppressed.
	w.maybeBroadcastLibraryUpdated(7)
	w.maybeBroadcastLibraryUpdated(7)

	if got := len(mb.snapshot()); got != 1 {
		t.Fatalf("expected throttled to 1 broadcast, got %d", got)
	}

	// A different library is not throttled by the first one's window.
	w.maybeBroadcastLibraryUpdated(8)
	if got := len(mb.snapshot()); got != 2 {
		t.Fatalf("expected per-library throttling, got %d total broadcasts", got)
	}

	// Waiting past the window lets the next emission through.
	time.Sleep(120 * time.Millisecond)
	w.maybeBroadcastLibraryUpdated(7)
	if got := len(mb.snapshot()); got != 3 {
		t.Fatalf("expected 3 broadcasts after window elapsed, got %d", got)
	}
}
