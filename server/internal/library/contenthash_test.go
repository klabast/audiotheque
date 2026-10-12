package library

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestContentHash(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return path
	}
	hash := func(path string) string {
		t.Helper()
		h, err := ContentHash(path)
		if err != nil {
			t.Fatalf("ContentHash(%s): %v", path, err)
		}
		return h
	}

	big := bytes.Repeat([]byte("abcdefgh"), 100_000)
	middleChanged := bytes.Clone(big)
	middleChanged[len(big)/2] = 'X'
	unsampledChanged := bytes.Clone(big)
	unsampledChanged[len(big)/4] = 'X'

	tests := []struct {
		name     string
		a, b     []byte
		wantSame bool
	}{
		{"identical bytes under another name", big, big, true},
		{"a byte changed in the middle sample", big, middleChanged, false},
		{"a byte changed outside the samples", big, unsampledChanged, true},
		{"one byte longer", big, append(bytes.Clone(big), 'a'), false},
		{"small files are hashed whole", []byte("tiny file"), []byte("tiny filf"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ha := hash(write("a.flac", tc.a))
			hb := hash(write("b.flac", tc.b))
			if same := ha == hb; same != tc.wantSame {
				t.Errorf("same hash = %v, want %v", same, tc.wantSame)
			}
		})
	}
}
