package mpd

import (
	"strings"
	"testing"

	"audiod/internal/mpd/testserver"
)

// TDD: GompdClient implements Client interface and works against fake MPD
func TestGompdClient_PlayFlow(t *testing.T) {
	// Start fake MPD
	srv := testserver.New()
	defer srv.Close()

	// Connect via GompdClient
	client, err := Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Failed to dial fake MPD: %v", err)
	}
	defer client.Close()

	// Load a URL
	err = client.LoadURL("http://localhost:8080/api/tracks/1/stream")
	if err != nil {
		t.Fatalf("LoadURL failed: %v", err)
	}

	// Play
	err = client.Play()
	if err != nil {
		t.Fatalf("Play failed: %v", err)
	}

	// Check status
	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.State != "play" {
		t.Errorf("Expected state 'play', got '%s'", status.State)
	}

	// Check current song
	song, err := client.CurrentSong()
	if err != nil {
		t.Fatalf("CurrentSong failed: %v", err)
	}
	if song.File != "http://localhost:8080/api/tracks/1/stream" {
		t.Errorf("Expected track URL, got '%s'", song.File)
	}

	// Pause
	err = client.Pause()
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	status, err = client.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.State != "pause" {
		t.Errorf("Expected state 'pause', got '%s'", status.State)
	}

	// Set volume
	err = client.SetVolume(50)
	if err != nil {
		t.Fatalf("SetVolume failed: %v", err)
	}
	status, err = client.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Volume != 50 {
		t.Errorf("Expected volume 50, got %d", status.Volume)
	}

	// Seek
	err = client.Seek(30)
	if err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	status, err = client.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Elapsed != 30 {
		t.Errorf("Expected elapsed 30, got %d", status.Elapsed)
	}

	// Stop
	err = client.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	status, err = client.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.State != "stop" {
		t.Errorf("Expected state 'stop', got '%s'", status.State)
	}

	// Verify via Go API
	state := srv.State()
	if state.PlayState != "stop" {
		t.Errorf("Expected Go API state 'stop', got '%s'", state.PlayState)
	}
}

// TDD: a HiFiBerry-style MPD with `mixer_type "none"` returns an ACK on
// setvol. We need the underlying error to surface the "No mixer" string so
// the device layer can recognize it and convert to ErrVolumeNotSupported.
func TestGompdClient_SetVolume_NoMixer(t *testing.T) {
	srv := testserver.New()
	defer srv.Close()
	srv.SetMixerless(true)

	client, err := Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	err = client.SetVolume(50)
	if err == nil {
		t.Fatal("expected SetVolume to fail on mixerless device")
	}
	if !strings.Contains(err.Error(), "No mixer") {
		t.Errorf("expected 'No mixer' in error, got: %v", err)
	}
}

// Compile-time check that GompdClient implements Client
var _ Client = (*GompdClient)(nil)
