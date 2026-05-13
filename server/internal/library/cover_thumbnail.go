package library

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/draw"

	// register WebP decoder when present (transitive); harmless if absent
	_ "golang.org/x/image/webp"
)

// CoverSize describes the rendered output size for a cover variant.
type CoverSize struct {
	Name string // "thumb" — used both as URL value and cache key
	Max  int    // longest-edge in pixels
}

// Predefined size presets. Keep this list closed so the on-disk cache stays
// finite and the API remains a small enum.
var coverSizes = map[string]CoverSize{
	"thumb": {Name: "thumb", Max: 400},
}

// LookupCoverSize returns the named preset or false if unknown. Callers must
// fall back to serving the original for unknown values.
func LookupCoverSize(name string) (CoverSize, bool) {
	s, ok := coverSizes[name]
	return s, ok
}

// CoverThumbnailer renders + caches scaled covers on disk. Concurrent renders
// of the same cache key collapse onto a single in-flight render via a tiny
// per-key mutex; otherwise the first scroll-burst could spawn dozens of
// duplicate decode/scale operations.
type CoverThumbnailer struct {
	cacheDir string

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewCoverThumbnailer returns a thumbnailer that caches under cacheDir.
func NewCoverThumbnailer(cacheDir string) *CoverThumbnailer {
	return &CoverThumbnailer{
		cacheDir: cacheDir,
		locks:    make(map[string]*sync.Mutex),
	}
}

// Resolve returns a path to a cached thumbnail for src at the requested size.
// If the thumbnail is missing it is rendered and persisted before returning.
// The returned path is always a regular file safe to serve via http.ServeFile.
func (t *CoverThumbnailer) Resolve(src string, size CoverSize, modTime int64) (string, error) {
	if t.cacheDir == "" {
		return "", fmt.Errorf("cover thumbnailer: no cache dir configured")
	}

	key := cacheKey(src, size.Name, modTime)
	out := filepath.Join(t.cacheDir, key+".jpg")

	if info, err := os.Stat(out); err == nil && !info.IsDir() && info.Size() > 0 {
		return out, nil
	}

	// Single-flight per cache key.
	keyLock := t.lockFor(key)
	keyLock.Lock()
	defer keyLock.Unlock()

	// Double-check under the lock — another goroutine may have rendered it
	// while we were queued.
	if info, err := os.Stat(out); err == nil && !info.IsDir() && info.Size() > 0 {
		return out, nil
	}

	if err := os.MkdirAll(t.cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	if err := renderJPEGThumbnail(src, out, size.Max); err != nil {
		return "", err
	}
	return out, nil
}

func (t *CoverThumbnailer) lockFor(key string) *sync.Mutex {
	t.mu.Lock()
	defer t.mu.Unlock()
	if l, ok := t.locks[key]; ok {
		return l
	}
	l := &sync.Mutex{}
	t.locks[key] = l
	return l
}

// cacheKey is a stable path-safe ID derived from source path + variant + the
// source's mtime. Bumping the source's mtime invalidates the cache for free.
func cacheKey(src, variant string, modTime int64) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s\x00%s\x00%d", src, variant, modTime)
	return hex.EncodeToString(h.Sum(nil))
}

// renderJPEGThumbnail decodes src (any format the std/x decoders register),
// scales it so the longest edge equals max (preserving aspect ratio), and
// writes it as JPEG to dst. Always quality 85 — the visible quality difference
// above that on album thumbnails is below the noise floor.
func renderJPEGThumbnail(src, dst string, max int) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	srcImg, _, err := image.Decode(in)
	if err != nil {
		return fmt.Errorf("decode source: %w", err)
	}

	bounds := srcImg.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	dstW, dstH := scaledSize(srcW, srcH, max)

	dstImg := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dstImg, dstImg.Rect, srcImg, bounds, draw.Over, nil)

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	if err := jpeg.Encode(out, dstImg, &jpeg.Options{Quality: 85}); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode jpeg: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close dst: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename dst: %w", err)
	}
	return nil
}

func scaledSize(srcW, srcH, max int) (int, int) {
	if srcW <= max && srcH <= max {
		return srcW, srcH
	}
	if srcW >= srcH {
		return max, max * srcH / srcW
	}
	return max * srcW / srcH, max
}
