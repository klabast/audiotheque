package library

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// contentHashSample is how much of the file's start, middle and end
// ContentHash reads. Files up to three samples long are hashed whole.
const contentHashSample = 64 * 1024

// ContentHash fingerprints a file by its size and three samples of its bytes,
// so a moved file can be recognised without reading all of it. It changes on
// retag, since tags live inside those bytes.
func ContentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()

	h := sha256.New()
	if err := binary.Write(h, binary.BigEndian, size); err != nil {
		return "", err
	}

	if size <= 3*contentHashSample {
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("hash %s: %w", path, err)
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}

	buf := make([]byte, contentHashSample)
	for _, offset := range []int64{0, size/2 - contentHashSample/2, size - contentHashSample} {
		if _, err := f.ReadAt(buf, offset); err != nil {
			return "", fmt.Errorf("hash %s at %d: %w", path, offset, err)
		}
		h.Write(buf)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
