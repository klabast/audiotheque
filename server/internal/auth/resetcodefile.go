package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"audiod/internal/config"
)

// resetCodeFiles persists the reset-code notification files the operator reads
// off disk (there is no mail transport). Keeping the file I/O behind this
// interface leaves Service free of filesystem calls and lets the cleanup job
// drop stale files the same way it drops stale rows.
type resetCodeFiles interface {
	Write(username, code string, createdAt time.Time) (string, error)
	DeleteExpired(now time.Time) (int, error)
}

// resetCodeFileStore writes into <data dir>/reset_codes. The directory is
// resolved per call rather than at construction so the store follows
// AUDIOD_DATA_DIR the same way the rest of the process does.
type resetCodeFileStore struct{}

func newResetCodeFileStore() resetCodeFiles { return &resetCodeFileStore{} }

func (s *resetCodeFileStore) dir() string {
	return filepath.Join(config.GetDataDir(), "reset_codes")
}

func (s *resetCodeFileStore) Write(username, code string, createdAt time.Time) (string, error) {
	dir := s.dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create reset codes directory: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%d_pw_reset_code_%s.json", createdAt.Unix(), username))
	body, err := json.MarshalIndent(map[string]string{
		"code":       code,
		"username":   username,
		"created_at": createdAt.Format(time.RFC3339),
		"expires_at": createdAt.Add(ResetCodeTTL).Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal reset code data: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write reset code file: %w", err)
	}
	return path, nil
}

// DeleteExpired removes notification files whose code can no longer be
// redeemed. Age is taken from the file's mtime: the file is written once, at
// the moment the code is issued, so mtime+ResetCodeTTL is exactly the code's
// expiry and needs no parsing of a possibly half-written file.
func (s *resetCodeFileStore) DeleteExpired(now time.Time) (int, error) {
	entries, err := os.ReadDir(s.dir())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read reset codes directory: %w", err)
	}

	removed := 0
	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Add(ResetCodeTTL).After(now) {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir(), entry.Name())); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("remove reset code file: %w", err)
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}
