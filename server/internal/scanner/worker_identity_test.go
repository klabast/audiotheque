package scanner

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"audiod/internal/library"
)

// identityHarness drives full scans over real files in a temp dir. File
// contents stand in for audio: the fake extractor maps a file's bytes to the
// tags registered for them, so a test controls both what the content hash sees
// and what the tags say.
type identityHarness struct {
	t     *testing.T
	w     *Worker
	repo  library.Repository
	db    *sql.DB
	lib   *library.Library
	roots []string
	tags  map[string]*library.AudioMetadata
	clock time.Time
}

func newIdentityHarness(t *testing.T, rootCount int) *identityHarness {
	t.Helper()
	w, repo, db, _ := setupTestWorker(t)

	var roots []string
	for i := 0; i < rootCount; i++ {
		roots = append(roots, t.TempDir())
	}
	lib, err := repo.CreateLibrary("identity", roots)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	h := &identityHarness{
		t: t, w: w, repo: repo, db: db, lib: lib, roots: roots,
		tags:  map[string]*library.AudioMetadata{},
		clock: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	w.extractMetadata = func(path string) (*library.AudioMetadata, error) {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		m, ok := h.tags[string(content)]
		if !ok {
			return nil, fmt.Errorf("no tags registered for %s", path)
		}
		copied := *m
		return &copied, nil
	}
	return h
}

func song(title string, trackNumber int) *library.AudioMetadata {
	return &library.AudioMetadata{
		Title: title, Artist: "Artist", AlbumArtist: "Artist", Album: "Album",
		TrackNumber: trackNumber, DiscNumber: 1,
		Duration: 200_000, SampleRate: 44100, BitDepth: 16, Channels: 2,
		Codec: "flac", IsLossless: true,
	}
}

func (h *identityHarness) abs(rel string) string {
	return filepath.Join(h.roots[0], rel)
}

// put writes content to rel (in the first root) and registers its tags. Each
// write gets a later mtime so the incremental scan always sees the change.
func (h *identityHarness) put(rel, content string, m *library.AudioMetadata) {
	h.putIn(0, rel, content, m)
}

func (h *identityHarness) putIn(root int, rel, content string, m *library.AudioMetadata) {
	h.t.Helper()
	h.tags[content] = m
	path := filepath.Join(h.roots[root], rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		h.t.Fatalf("write %s: %v", rel, err)
	}
	h.clock = h.clock.Add(time.Minute)
	if err := os.Chtimes(path, h.clock, h.clock); err != nil {
		h.t.Fatalf("chtimes: %v", err)
	}
}

func (h *identityHarness) move(from, to string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.abs(to)), 0o755); err != nil {
		h.t.Fatalf("mkdir: %v", err)
	}
	if err := os.Rename(h.abs(from), h.abs(to)); err != nil {
		h.t.Fatalf("move %s → %s: %v", from, to, err)
	}
}

func (h *identityHarness) remove(rel string) {
	h.t.Helper()
	if err := os.RemoveAll(h.abs(rel)); err != nil {
		h.t.Fatalf("remove %s: %v", rel, err)
	}
}

func (h *identityHarness) scan() {
	h.t.Helper()
	if _, err := h.repo.QueueScan(h.lib.ID); err != nil {
		h.t.Fatalf("QueueScan: %v", err)
	}
	h.w.processPendingScans()
}

func (h *identityHarness) track(rel string) *library.Track {
	h.t.Helper()
	return h.trackAt(h.abs(rel))
}

func (h *identityHarness) trackAt(path string) *library.Track {
	h.t.Helper()
	tr, err := h.repo.GetTrackByPath(h.lib.ID, path)
	if err != nil {
		h.t.Fatalf("no track at %s: %v", path, err)
	}
	return tr
}

func (h *identityHarness) count(query string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.db.QueryRow(query, args...).Scan(&n); err != nil {
		h.t.Fatalf("count: %v", err)
	}
	return n
}

func (h *identityHarness) trackCount() int {
	return h.count(`SELECT COUNT(*) FROM track WHERE library_id = ?`, h.lib.ID)
}

func (h *identityHarness) albumCount() int {
	return h.count(`SELECT COUNT(*) FROM album WHERE library_id = ?`, h.lib.ID)
}

func TestScan_TrackIdentity(t *testing.T) {
	withMBID := func(m *library.AudioMetadata, id string) *library.AudioMetadata {
		m.MusicBrainzTrackID = id
		return m
	}
	retitled := func(m *library.AudioMetadata, title string) *library.AudioMetadata {
		m.Title = title
		return m
	}
	regenred := func(m *library.AudioMetadata) *library.AudioMetadata {
		m.Genre = "Jazz"
		return m
	}
	lossy := func(m *library.AudioMetadata, durationDelta int) *library.AudioMetadata {
		m.Codec, m.IsLossless, m.Duration = "mp3", false, m.Duration+durationDelta
		return m
	}

	cases := []struct {
		name     string
		before   *library.AudioMetadata
		change   func(h *identityHarness)
		newPath  string
		sameID   bool
		oldStays bool
	}{
		{
			name:   "retag in place keeps the track",
			before: song("Song", 1),
			change: func(h *identityHarness) {
				h.put("Album/01.flac", "song-retagged", regenred(song("Song", 1)))
			},
			newPath: "Album/01.flac",
			sameID:  true,
		},
		{
			name:    "rename keeps the track",
			before:  song("Song", 1),
			change:  func(h *identityHarness) { h.move("Album/01.flac", "Album/01 - Song.flac") },
			newPath: "Album/01 - Song.flac",
			sameID:  true,
		},
		{
			name:    "folder move keeps the track",
			before:  song("Song", 1),
			change:  func(h *identityHarness) { h.move("Album", "Artist/Album") },
			newPath: "Artist/Album/01.flac",
			sameID:  true,
		},
		{
			name:   "retag and rename keeps the track by recording id",
			before: withMBID(song("Song", 1), "rec-1"),
			change: func(h *identityHarness) {
				h.remove("Album/01.flac")
				h.put("Artist/Album/01 Song (Remastered).flac", "song-picard",
					withMBID(retitled(song("Song", 1), "Song (Remastered)"), "rec-1"))
			},
			newPath: "Artist/Album/01 Song (Remastered).flac",
			sameID:  true,
		},
		{
			name:   "retag and rename without recording id keeps the track by its tags",
			before: song("Song", 1),
			change: func(h *identityHarness) {
				h.remove("Album/01.flac")
				h.put("Artist/Album/01 Song.flac", "song-picard", regenred(song("Song", 1)))
			},
			newPath: "Artist/Album/01 Song.flac",
			sameID:  true,
		},
		{
			name:   "replacement rip at a new path keeps the track",
			before: song("Song", 1),
			change: func(h *identityHarness) {
				h.remove("Album/01.flac")
				h.put("Album/01.mp3", "song-mp3", lossy(song("Song", 1), 1500))
			},
			newPath: "Album/01.mp3",
			sameID:  true,
		},
		{
			name:   "same tags but a clearly different length is a new track",
			before: song("Song", 1),
			change: func(h *identityHarness) {
				h.remove("Album/01.flac")
				h.put("Album/01 (Live).flac", "song-live", lossy(song("Song", 1), 30_000))
			},
			newPath: "Album/01 (Live).flac",
			sameID:  false,
		},
		{
			name:   "a copy is a second track",
			before: song("Song", 1),
			change: func(h *identityHarness) {
				h.put("Copy/01.flac", "song", song("Song", 1))
			},
			newPath:  "Copy/01.flac",
			sameID:   false,
			oldStays: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given a scanned track
			h := newIdentityHarness(t, 1)
			h.put("Album/01.flac", "song", tc.before)
			h.scan()
			original := h.track("Album/01.flac").ID

			// When the library changes and is scanned again
			tc.change(h)
			h.scan()

			// Then the track at the new path has the expected identity
			got := h.track(tc.newPath).ID
			if tc.sameID && got != original {
				t.Errorf("track id = %d, want %d (identity lost)", got, original)
			}
			if !tc.sameID && got == original {
				t.Errorf("track id = %d, want a new track", got)
			}
			wantTracks := 1
			if tc.oldStays {
				wantTracks = 2
			}
			if n := h.trackCount(); n != wantTracks {
				t.Errorf("tracks = %d, want %d", n, wantTracks)
			}
		})
	}
}

func TestScan_TrackIdentity_AmbiguousMatchMakesANewTrack(t *testing.T) {
	// Given two tracks that share every tag but are different files
	h := newIdentityHarness(t, 1)
	h.put("A/01.flac", "take-a", song("Song", 1))
	h.put("B/01.flac", "take-b", song("Song", 1))
	h.scan()
	idA, idB := h.track("A/01.flac").ID, h.track("B/01.flac").ID

	// When both vanish and one new file with the same tags appears
	h.remove("A")
	h.remove("B")
	h.put("C/01.flac", "take-c", song("Song", 1))
	h.scan()

	// Then neither old track is guessed at
	got := h.track("C/01.flac").ID
	if got == idA || got == idB {
		t.Errorf("track id = %d, want a new track (candidates %d, %d)", got, idA, idB)
	}
}

func TestScan_TrackIdentity_UnreadableRootIsNotAMove(t *testing.T) {
	// Given a track in the second library path
	h := newIdentityHarness(t, 2)
	h.putIn(1, "Album/01.flac", "song", song("Song", 1))
	h.scan()
	oldPath := filepath.Join(h.roots[1], "Album/01.flac")
	original := h.trackAt(oldPath).ID

	// When that path becomes unreadable and an identical file shows up in the first
	if err := os.RemoveAll(h.roots[1]); err != nil {
		t.Fatalf("remove root: %v", err)
	}
	h.put("Album/01.flac", "song", song("Song", 1))
	h.scan()

	// Then the old track is left alone and the new file is its own track
	if got := h.track("Album/01.flac").ID; got == original {
		t.Errorf("track id = %d, relinked across an unreadable library path", got)
	}
	if got := h.trackAt(oldPath).ID; got != original {
		t.Errorf("old track id = %d, want %d", got, original)
	}
}

func TestScan_AlbumIdentity(t *testing.T) {
	release := func(m *library.AudioMetadata, mbid string) *library.AudioMetadata {
		m.MusicBrainzAlbumID = mbid
		return m
	}

	t.Run("folder move keeps the album and leaves no empty album behind", func(t *testing.T) {
		// Given a scanned album
		h := newIdentityHarness(t, 1)
		h.put("Album/01.flac", "one", song("One", 1))
		h.put("Album/02.flac", "two", song("Two", 2))
		h.scan()
		original := *h.track("Album/01.flac").AlbumID

		// When its folder moves
		h.move("Album", "Artist/Album")
		h.scan()

		// Then both tracks still belong to the same album, and it is the only one
		for _, rel := range []string{"Artist/Album/01.flac", "Artist/Album/02.flac"} {
			if got := *h.track(rel).AlbumID; got != original {
				t.Errorf("%s album id = %d, want %d", rel, got, original)
			}
		}
		if n := h.albumCount(); n != 1 {
			t.Errorf("albums = %d, want 1", n)
		}
	})

	t.Run("two editions of one release stay two albums", func(t *testing.T) {
		// Given the same release in two folders
		h := newIdentityHarness(t, 1)
		h.put("Album/01.flac", "cd", release(song("One", 1), "rel-1"))
		h.put("Album [24-96]/01.flac", "hires", release(song("One", 1), "rel-1"))

		// When the library is scanned
		h.scan()

		// Then each folder is its own album
		if a, b := *h.track("Album/01.flac").AlbumID, *h.track("Album [24-96]/01.flac").AlbumID; a == b {
			t.Errorf("both editions share album %d", a)
		}
	})

	t.Run("retitled in place keeps the album by release id", func(t *testing.T) {
		// Given an album tagged with a release id
		h := newIdentityHarness(t, 1)
		h.put("Album/01.flac", "one", release(song("One", 1), "rel-1"))
		h.scan()
		original := *h.track("Album/01.flac").AlbumID

		// When its title tag changes in place
		retitled := release(song("One", 1), "rel-1")
		retitled.Album = "Album (Deluxe)"
		h.put("Album/01.flac", "one-deluxe", retitled)
		h.scan()

		// Then it is still the same album
		if got := *h.track("Album/01.flac").AlbumID; got != original {
			t.Errorf("album id = %d, want %d", got, original)
		}
		if n := h.albumCount(); n != 1 {
			t.Errorf("albums = %d, want 1", n)
		}
	})

	t.Run("an album whose files are all gone is removed", func(t *testing.T) {
		// Given two scanned albums
		h := newIdentityHarness(t, 1)
		h.put("Keep/01.flac", "keep", song("Keep", 1))
		gone := song("Gone", 1)
		gone.Album = "Gone"
		h.put("Gone/01.flac", "gone", gone)
		h.scan()

		// When one album's folder is deleted
		h.remove("Gone")
		h.scan()

		// Then only the other album is left
		if n := h.albumCount(); n != 1 {
			t.Errorf("albums = %d, want 1", n)
		}
	})

	t.Run("the release id is saved", func(t *testing.T) {
		// Given an album tagged with a release id
		h := newIdentityHarness(t, 1)
		h.put("Album/01.flac", "one", release(song("One", 1), "rel-1"))

		// When it is scanned
		h.scan()

		// Then the album carries it
		album, err := h.repo.GetAlbumByID(*h.track("Album/01.flac").AlbumID)
		if err != nil {
			t.Fatalf("GetAlbumByID: %v", err)
		}
		if album.MusicBrainzID != "rel-1" {
			t.Errorf("album musicbrainz id = %q, want %q", album.MusicBrainzID, "rel-1")
		}
	})
}
