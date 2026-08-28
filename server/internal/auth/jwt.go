package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"audiod/internal/config"
)

// MinJWTSecretLength is the shortest HMAC key we will sign or verify with.
// Anything shorter (a truncated file, a zero-byte file, JWT_SECRET=x) makes
// stream tokens forgeable for any track and any user.
const MinJWTSecretLength = 32

const jwtSecretFilename = "jwt-secret.key"

// jwtSecretMu serialises first-run generation so two concurrent requests
// can't each generate and write a different secret.
var jwtSecretMu sync.Mutex

// EnsureJWTSecret resolves the signing secret, generating and persisting one
// on first run. Call it once at startup: without it the first resolution
// happens inside a live request, where a disk problem surfaces as a failed
// stream rather than a failed boot.
func EnsureJWTSecret() error {
	_, err := getJWTSecret()
	return err
}

func getJWTSecret() ([]byte, error) {
	jwtSecretMu.Lock()
	defer jwtSecretMu.Unlock()

	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		if len(secret) < MinJWTSecretLength {
			return nil, fmt.Errorf("JWT_SECRET is %d bytes, need at least %d", len(secret), MinJWTSecretLength)
		}
		return []byte(secret), nil
	}

	dataDir := config.GetDataDir()
	secretPath := filepath.Join(dataDir, jwtSecretFilename)

	data, err := os.ReadFile(secretPath)
	switch {
	case err == nil:
		if len(data) < MinJWTSecretLength {
			return nil, fmt.Errorf("jwt secret %s is %d bytes, need at least %d", secretPath, len(data), MinJWTSecretLength)
		}
		return data, nil
	case !errors.Is(err, fs.ErrNotExist):
		// Only "the file isn't there" may lead to generation. Any other read
		// failure (permissions, fd exhaustion) can be transient, and writing a
		// fresh secret over a still-present one would invalidate every
		// outstanding token permanently.
		return nil, fmt.Errorf("read jwt secret %s: %w", secretPath, err)
	}

	newSecret := make([]byte, MinJWTSecretLength)
	if _, err := rand.Read(newSecret); err != nil {
		return nil, fmt.Errorf("generate jwt secret: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir %s: %w", dataDir, err)
	}
	if err := os.WriteFile(secretPath, newSecret, 0o600); err != nil {
		return nil, fmt.Errorf("write jwt secret %s: %w", secretPath, err)
	}

	log.Printf("Generated a new JWT secret at %s - back this file up", secretPath)
	return newSecret, nil
}
