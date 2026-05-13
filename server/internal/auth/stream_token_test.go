package auth

import (
	"os"
	"testing"
	"time"
)

func setStreamTestSecret(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-for-stream-tokens-do-not-use-in-prod")
	// Ensure data-dir fallback never runs during these tests.
	_ = os.Unsetenv("AUDIOD_DATA_DIR")
}

// Round-trip: a freshly minted token validates and returns the user ID.
func TestMintAndValidateStreamToken(t *testing.T) {
	setStreamTestSecret(t)

	token, err := MintStreamToken(42, 7, time.Hour)
	if err != nil {
		t.Fatalf("MintStreamToken: %v", err)
	}
	userID, err := ValidateStreamToken(token, 42)
	if err != nil {
		t.Fatalf("ValidateStreamToken: %v", err)
	}
	if userID != 7 {
		t.Errorf("expected userID 7, got %d", userID)
	}
}

// A token minted for one trackID must not validate against another.
func TestValidateStreamToken_TrackIDMismatch(t *testing.T) {
	setStreamTestSecret(t)

	token, err := MintStreamToken(42, 7, time.Hour)
	if err != nil {
		t.Fatalf("MintStreamToken: %v", err)
	}
	if _, err := ValidateStreamToken(token, 99); err == nil {
		t.Fatal("expected error for mismatched trackID, got nil")
	}
}

// An expired token is rejected.
func TestValidateStreamToken_Expired(t *testing.T) {
	setStreamTestSecret(t)

	token, err := MintStreamToken(42, 7, -time.Second) // already expired
	if err != nil {
		t.Fatalf("MintStreamToken: %v", err)
	}
	if _, err := ValidateStreamToken(token, 42); err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

// A regular session JWT (no `kind: stream`) must not be accepted as a stream
// token, even though it's signed with the same secret.
func TestValidateStreamToken_RejectsSessionJWT(t *testing.T) {
	setStreamTestSecret(t)

	sessionToken, err := GenerateToken(7, "alice")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := ValidateStreamToken(sessionToken, 42); err == nil {
		t.Fatal("expected error when validating session JWT as stream token")
	}
}

// Empty token is rejected without panicking.
func TestValidateStreamToken_Empty(t *testing.T) {
	setStreamTestSecret(t)
	if _, err := ValidateStreamToken("", 42); err == nil {
		t.Fatal("expected error for empty token")
	}
}

// Garbage input is rejected.
func TestValidateStreamToken_Garbage(t *testing.T) {
	setStreamTestSecret(t)
	if _, err := ValidateStreamToken("not-a-jwt", 42); err == nil {
		t.Fatal("expected error for garbage token")
	}
}
