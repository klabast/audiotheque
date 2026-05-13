package library

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverThumbnailer_RendersAndCaches(t *testing.T) {
	dir := t.TempDir()

	// Source: 800x600 JPEG.
	srcPath := filepath.Join(dir, "src.jpg")
	writeTestJPEG(t, srcPath, 800, 600)

	cache := filepath.Join(dir, "cache")
	tn := NewCoverThumbnailer(cache)

	preset, ok := LookupCoverSize("thumb")
	if !ok {
		t.Fatal("thumb preset missing")
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("stat src: %v", err)
	}
	mod := srcInfo.ModTime().UnixNano()

	// First call: renders.
	out1, err := tn.Resolve(srcPath, preset, mod)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !fileExists(out1) {
		t.Fatalf("thumbnail not written: %s", out1)
	}

	// Verify scaled size: longest edge == preset.Max, aspect preserved.
	w, h := decodeJPEGSize(t, out1)
	if w != preset.Max {
		t.Errorf("width: want %d, got %d", preset.Max, w)
	}
	if h != preset.Max*600/800 {
		t.Errorf("height: want %d, got %d", preset.Max*600/800, h)
	}

	// Second call with same key: cache hit, same path, no re-render.
	stat1, _ := os.Stat(out1)
	out2, err := tn.Resolve(srcPath, preset, mod)
	if err != nil {
		t.Fatalf("resolve cached: %v", err)
	}
	if out1 != out2 {
		t.Errorf("expected stable cache path; got %q vs %q", out1, out2)
	}
	stat2, _ := os.Stat(out2)
	if !stat1.ModTime().Equal(stat2.ModTime()) {
		t.Error("cache hit should not re-render the thumbnail")
	}

	// Bumping mtime changes the cache key; re-render goes to a new file.
	out3, err := tn.Resolve(srcPath, preset, mod+1)
	if err != nil {
		t.Fatalf("resolve bumped: %v", err)
	}
	if out3 == out1 {
		t.Error("expected new cache key when mtime changes")
	}
}

func TestCoverThumbnailer_NoUpscale(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "tiny.jpg")
	writeTestJPEG(t, srcPath, 100, 80)

	tn := NewCoverThumbnailer(filepath.Join(dir, "cache"))
	preset, _ := LookupCoverSize("thumb")
	info, _ := os.Stat(srcPath)
	out, err := tn.Resolve(srcPath, preset, info.ModTime().UnixNano())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Source is smaller than preset.Max — no upscaling, dims preserved.
	w, h := decodeJPEGSize(t, out)
	if w != 100 || h != 80 {
		t.Errorf("expected no upscale; want 100x80, got %dx%d", w, h)
	}
}

func TestLookupCoverSize(t *testing.T) {
	if _, ok := LookupCoverSize("thumb"); !ok {
		t.Error("thumb preset must be registered")
	}
	if _, ok := LookupCoverSize("bogus"); ok {
		t.Error("unknown preset must return ok=false")
	}
}

// --- helpers ---

func writeTestJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with a recognizable color so visual debugging stays sane.
	c := color.RGBA{R: 200, G: 100, B: 50, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

func decodeJPEGSize(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return cfg.Width, cfg.Height
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
