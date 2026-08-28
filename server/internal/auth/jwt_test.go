package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A truncated or zero-byte secret file yields an empty HMAC key, which lets
// anyone forge a stream token for any track. Signing must fail instead.
func TestJWTSecret_TruncatedFileIsRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AUDIOD_DATA_DIR", dir)
	t.Setenv("JWT_SECRET", "")

	if err := os.WriteFile(filepath.Join(dir, "jwt-secret.key"), []byte("short"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	if _, err := MintStreamToken(1, 2, time.Hour); err == nil {
		t.Fatal("expected an error for a truncated jwt secret, got nil")
	}
}

// An empty file is the degenerate case of the same bug.
func TestJWTSecret_EmptyFileIsRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AUDIOD_DATA_DIR", dir)
	t.Setenv("JWT_SECRET", "")

	if err := os.WriteFile(filepath.Join(dir, "jwt-secret.key"), nil, 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	if _, err := MintStreamToken(1, 2, time.Hour); err == nil {
		t.Fatal("expected an error for an empty jwt secret, got nil")
	}
}

// A read failure that isn't "no such file" (permissions, fd exhaustion, the
// wrong working directory) must never lead to generating over a secret that
// is still there: every outstanding token would die with it.
func TestJWTSecret_ReadFailureDoesNotOverwrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a 0000 file, so this failure mode can't be simulated")
	}
	dir := t.TempDir()
	t.Setenv("AUDIOD_DATA_DIR", dir)
	t.Setenv("JWT_SECRET", "")

	path := filepath.Join(dir, "jwt-secret.key")
	original := bytes.Repeat([]byte("k"), MinJWTSecretLength)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	if _, err := MintStreamToken(1, 2, time.Hour); err == nil {
		t.Fatal("expected an error when the secret can't be read, got nil")
	}

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod back: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Error("the existing secret was overwritten after a transient read failure")
	}
}

// First run still works: no file, no env var, one gets generated and persisted.
func TestJWTSecret_GeneratesOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AUDIOD_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("JWT_SECRET", "")

	if err := EnsureJWTSecret(); err != nil {
		t.Fatalf("EnsureJWTSecret: %v", err)
	}

	secret, err := os.ReadFile(filepath.Join(dir, "data", "jwt-secret.key"))
	if err != nil {
		t.Fatalf("read generated secret: %v", err)
	}
	if len(secret) < MinJWTSecretLength {
		t.Errorf("generated secret is %d bytes, want at least %d", len(secret), MinJWTSecretLength)
	}

	// A second call must reuse it, not rotate it.
	if err := EnsureJWTSecret(); err != nil {
		t.Fatalf("second EnsureJWTSecret: %v", err)
	}
	again, err := os.ReadFile(filepath.Join(dir, "data", "jwt-secret.key"))
	if err != nil {
		t.Fatalf("re-read secret: %v", err)
	}
	if !bytes.Equal(secret, again) {
		t.Error("the secret was rotated on a second resolution")
	}
}

// A short JWT_SECRET env var is just as forgeable as a short file.
func TestJWTSecret_ShortEnvVarIsRejected(t *testing.T) {
	t.Setenv("AUDIOD_DATA_DIR", t.TempDir())
	t.Setenv("JWT_SECRET", "x")

	if _, err := MintStreamToken(1, 2, time.Hour); err == nil {
		t.Fatal("expected an error for a one-byte JWT_SECRET, got nil")
	}
}
