package auth

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedUserWithPassword adds a user to the mock repository with a real hash.
func seedUserWithPassword(t *testing.T, repo *mockRepository, id int64, username, password string, isAdmin bool) *User {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &User{ID: id, Username: username, PasswordHash: hash, IsAdmin: isAdmin}
	repo.users[username] = u
	repo.usersID[id] = u
	return u
}

// A stolen cookie must stop working the moment the victim resets their
// password — the one remediation a user knows to perform.
func TestConfirmPasswordReset_RevokesExistingSessions(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	user := seedUserWithPassword(t, repo, 1, "alice", "oldpassword", false)

	stolen, err := service.CreateSession(user.ID, SessionContext{})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	code, err := service.RequestPasswordReset("alice")
	if err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if _, err := service.ConfirmPasswordReset(code, "newpassword"); err != nil {
		t.Fatalf("ConfirmPasswordReset: %v", err)
	}

	if _, _, err := service.ValidateSession(stolen, "10.0.0.1"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("session survived a password reset: err=%v", err)
	}
}

// An admin resetting a user's password must knock that user's devices out too.
func TestAdminResetUserPassword_RevokesExistingSessions(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	user := seedUserWithPassword(t, repo, 2, "bob", "bobpassword", false)

	stolen, err := service.CreateSession(user.ID, SessionContext{})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := service.AdminResetUserPassword(user.ID, "bobnewpassword"); err != nil {
		t.Fatalf("AdminResetUserPassword: %v", err)
	}

	if _, _, err := service.ValidateSession(stolen, "10.0.0.1"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("session survived an admin password reset: err=%v", err)
	}
}

// The reset code is password-equivalent: anyone with log access must not be
// able to take over an account, and the trigger is unauthenticated.
func TestRequestPasswordResetWithFile_DoesNotLogTheCode(t *testing.T) {
	t.Setenv("AUDIOD_DATA_DIR", t.TempDir())
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	seedUserWithPassword(t, repo, 1, "alice", "alicepassword", false)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	if err := service.RequestPasswordResetWithFile("alice"); err != nil {
		t.Fatalf("RequestPasswordResetWithFile: %v", err)
	}

	var code string
	for c := range repo.resetCodes {
		code = c
	}
	if code == "" {
		t.Fatal("no reset code was stored")
	}
	if strings.Contains(buf.String(), code) {
		t.Errorf("reset code leaked into the log: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "alice") {
		t.Errorf("expected the username in the log, got: %s", buf.String())
	}
}

// Changing the password must not leave a stolen cookie working, and must not
// log the caller out of the tab they changed it from.
func TestUpdatePassword_RevokesOldSessionsAndOpensANewOne(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	user := seedUserWithPassword(t, repo, 1, "alice", "oldpassword", false)

	stolen, err := service.CreateSession(user.ID, SessionContext{})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	fresh, err := service.UpdatePassword(user.ID, "oldpassword", "newpassword", SessionContext{})
	if err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	if _, _, err := service.ValidateSession(stolen, "10.0.0.1"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("old session survived a password change: err=%v", err)
	}
	if fresh == "" || fresh == stolen {
		t.Fatalf("expected a brand-new session id, got %q", fresh)
	}
	if _, _, err := service.ValidateSession(fresh, "10.0.0.1"); err != nil {
		t.Errorf("replacement session is not usable: %v", err)
	}
}

// An unknown username used to return before any hashing happened, which told
// a caller with a stopwatch exactly which accounts exist.
func TestAuthenticate_UnknownUserCostsTheSameAsAWrongPassword(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	seedUserWithPassword(t, repo, 1, "alice", "alicepassword", false)

	// Warm the decoy hash so its one-time construction isn't counted.
	_, _ = service.Authenticate("nosuchuser", "whatever")

	start := time.Now()
	_, _ = service.Authenticate("alice", "wrongpassword")
	known := time.Since(start)

	start = time.Now()
	_, _ = service.Authenticate("nosuchuser", "wrongpassword")
	unknown := time.Since(start)

	if unknown < known/2 {
		t.Errorf("unknown user answered in %v vs %v for a known one — the gap identifies valid usernames", unknown, known)
	}
}

// Setup conflicts are matched with errors.Is, not by comparing message text.
func TestCreateFirstUser_SecondCallReturnsSentinel(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	if _, _, err := service.CreateFirstUser("alice", "alicepassword", SessionContext{}); err != nil {
		t.Fatalf("first setup failed: %v", err)
	}
	_, _, err := service.CreateFirstUser("mallory", "mallorypassword", SessionContext{})
	if !errors.Is(err, ErrSetupAlreadyCompleted) {
		t.Errorf("got %v, want ErrSetupAlreadyCompleted", err)
	}
}

// The hourly job is the only thing that ever touches these files outside a
// full CLI reset.
func TestCleanupExpiredResetCodes_RemovesStaleFiles(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("AUDIOD_DATA_DIR", dataDir)

	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	seedUserWithPassword(t, repo, 1, "alice", "alicepassword", false)

	if err := service.RequestPasswordResetWithFile("alice"); err != nil {
		t.Fatalf("RequestPasswordResetWithFile: %v", err)
	}

	resetDir := filepath.Join(dataDir, "reset_codes")
	entries, err := os.ReadDir(resetDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one reset code file, got %v (err=%v)", entries, err)
	}

	stale := filepath.Join(resetDir, entries[0].Name())
	old := time.Now().Add(-2 * ResetCodeTTL)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("age the file: %v", err)
	}

	if err := service.CleanupExpiredResetCodes(); err != nil {
		t.Fatalf("CleanupExpiredResetCodes: %v", err)
	}

	entries, err = os.ReadDir(resetDir)
	if err != nil {
		t.Fatalf("read reset dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expired reset code file was left behind: %v", entries[0].Name())
	}
}

// A file for a code that is still redeemable stays put.
func TestCleanupExpiredResetCodes_KeepsLiveFiles(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("AUDIOD_DATA_DIR", dataDir)

	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	seedUserWithPassword(t, repo, 1, "alice", "alicepassword", false)

	if err := service.RequestPasswordResetWithFile("alice"); err != nil {
		t.Fatalf("RequestPasswordResetWithFile: %v", err)
	}
	if err := service.CleanupExpiredResetCodes(); err != nil {
		t.Fatalf("CleanupExpiredResetCodes: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, "reset_codes"))
	if err != nil {
		t.Fatalf("read reset dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected the live reset code file to survive, got %d files", len(entries))
	}
}

// Admin-facing service entry points carry the same rules as the handlers.
func TestServiceValidation_AppliesToAdminEntryPoints(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	seedUserWithPassword(t, repo, 1, "alice", "alicepassword", true)

	tooLong := strings.Repeat("p", MaxPasswordLength+1)

	if _, err := service.CreateUser("b", "bobpassword", false); err == nil {
		t.Error("CreateUser accepted a one-character username")
	}
	if _, err := service.CreateUser("bob", tooLong, false); err == nil {
		t.Error("CreateUser accepted an over-long password")
	}
	if err := service.AdminResetUserPassword(1, tooLong); err == nil {
		t.Error("AdminResetUserPassword accepted an over-long password")
	}
}
