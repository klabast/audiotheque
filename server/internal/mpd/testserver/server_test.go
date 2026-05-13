package testserver

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
)

// helper: connect to fake MPD, read greeting, return conn + scanner
func connect(t *testing.T, addr string) (net.Conn, *bufio.Scanner) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to fake MPD: %v", err)
	}
	scanner := bufio.NewScanner(conn)

	// Read MPD greeting line
	if !scanner.Scan() {
		t.Fatal("Expected MPD greeting")
	}
	greeting := scanner.Text()
	if !strings.HasPrefix(greeting, "OK MPD") {
		t.Fatalf("Expected 'OK MPD ...' greeting, got: %s", greeting)
	}

	return conn, scanner
}

// helper: send command, read all response lines until "OK" or "ACK"
func sendCommand(t *testing.T, conn net.Conn, scanner *bufio.Scanner, cmd string) []string {
	t.Helper()
	_, err := fmt.Fprintf(conn, "%s\n", cmd)
	if err != nil {
		t.Fatalf("Failed to send command: %v", err)
	}

	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "OK" || strings.HasPrefix(line, "ACK") {
			lines = append(lines, line)
			break
		}
		lines = append(lines, line)
	}
	return lines
}

// helper: parse "key: value" lines into a map
func parseResponse(lines []string) map[string]string {
	result := make(map[string]string)
	for _, line := range lines {
		if line == "OK" || strings.HasPrefix(line, "ACK") {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// TDD: Fake MPD server accepts connections and sends greeting
func TestServer_AcceptsConnection(t *testing.T) {
	srv := New()
	defer srv.Close()

	addr := srv.Addr()
	conn, _ := connect(t, addr)
	defer conn.Close()
}

// TDD: Fake MPD server responds to 'status' command with stopped state
func TestServer_Status_InitialState(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	lines := sendCommand(t, conn, scanner, "status")
	status := parseResponse(lines)

	if status["state"] != "stop" {
		t.Errorf("Expected initial state 'stop', got '%s'", status["state"])
	}
	if status["volume"] != "100" {
		t.Errorf("Expected initial volume '100', got '%s'", status["volume"])
	}
}

// TDD: Fake MPD can load a URL and play it
func TestServer_LoadAndPlay(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	// Clear and add a URL
	lines := sendCommand(t, conn, scanner, "clear")
	if lines[len(lines)-1] != "OK" {
		t.Errorf("Expected OK for clear, got: %v", lines)
	}

	lines = sendCommand(t, conn, scanner, "add http://localhost:8080/api/tracks/1/stream")
	if lines[len(lines)-1] != "OK" {
		t.Errorf("Expected OK for add, got: %v", lines)
	}

	lines = sendCommand(t, conn, scanner, "play")
	if lines[len(lines)-1] != "OK" {
		t.Errorf("Expected OK for play, got: %v", lines)
	}

	// Check status
	lines = sendCommand(t, conn, scanner, "status")
	status := parseResponse(lines)
	if status["state"] != "play" {
		t.Errorf("Expected state 'play', got '%s'", status["state"])
	}

	// Check current song
	lines = sendCommand(t, conn, scanner, "currentsong")
	song := parseResponse(lines)
	if song["file"] != "http://localhost:8080/api/tracks/1/stream" {
		t.Errorf("Expected file URL, got '%s'", song["file"])
	}
}

// TDD: Fake MPD handles pause/resume
func TestServer_PauseResume(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	// Start playing
	sendCommand(t, conn, scanner, "add http://example.com/track.mp3")
	sendCommand(t, conn, scanner, "play")

	// Pause
	lines := sendCommand(t, conn, scanner, "pause 1")
	if lines[len(lines)-1] != "OK" {
		t.Errorf("Expected OK for pause, got: %v", lines)
	}

	lines = sendCommand(t, conn, scanner, "status")
	status := parseResponse(lines)
	if status["state"] != "pause" {
		t.Errorf("Expected state 'pause', got '%s'", status["state"])
	}

	// Resume
	sendCommand(t, conn, scanner, "pause 0")
	lines = sendCommand(t, conn, scanner, "status")
	status = parseResponse(lines)
	if status["state"] != "play" {
		t.Errorf("Expected state 'play', got '%s'", status["state"])
	}
}

// TDD: Fake MPD handles stop
func TestServer_Stop(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	sendCommand(t, conn, scanner, "add http://example.com/track.mp3")
	sendCommand(t, conn, scanner, "play")

	lines := sendCommand(t, conn, scanner, "stop")
	if lines[len(lines)-1] != "OK" {
		t.Errorf("Expected OK for stop, got: %v", lines)
	}

	lines = sendCommand(t, conn, scanner, "status")
	status := parseResponse(lines)
	if status["state"] != "stop" {
		t.Errorf("Expected state 'stop', got '%s'", status["state"])
	}
}

// TDD: Fake MPD handles setvol
func TestServer_SetVolume(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	lines := sendCommand(t, conn, scanner, "setvol 50")
	if lines[len(lines)-1] != "OK" {
		t.Errorf("Expected OK for setvol, got: %v", lines)
	}

	lines = sendCommand(t, conn, scanner, "status")
	status := parseResponse(lines)
	if status["volume"] != "50" {
		t.Errorf("Expected volume '50', got '%s'", status["volume"])
	}
}

// TDD: Fake MPD handles seek
func TestServer_Seek(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	sendCommand(t, conn, scanner, "add http://example.com/track.mp3")
	sendCommand(t, conn, scanner, "play")

	lines := sendCommand(t, conn, scanner, "seekcur 45")
	if lines[len(lines)-1] != "OK" {
		t.Errorf("Expected OK for seekcur, got: %v", lines)
	}

	lines = sendCommand(t, conn, scanner, "status")
	status := parseResponse(lines)
	if status["elapsed"] != "45" {
		t.Errorf("Expected elapsed '45', got '%s'", status["elapsed"])
	}
}

// SetMixerless toggles HiFiBerry-style "no mixer" mode where setvol returns
// the same ACK string real MPD returns when the audio output has
// `mixer_type "none"`. Used by tests that need to validate audiod's tolerance
// of mixerless devices.
func TestServer_SetMixerless_RejectsSetVolWithNoMixerACK(t *testing.T) {
	srv := New()
	defer srv.Close()
	srv.SetMixerless(true)

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	lines := sendCommand(t, conn, scanner, "setvol 50")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "ACK") {
		t.Fatalf("expected ACK for mixerless setvol, got: %v", lines)
	}
	if !strings.Contains(last, "No mixer") {
		t.Errorf("expected ACK to mention 'No mixer', got: %s", last)
	}
}

// In real MPD, every `add` increments songid. The fake server must do the
// same so tests can detect track transitions via songid change rather than
// fragile state-machine timing.
func TestServer_SongIDIncrementsPerAdd(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	sendCommand(t, conn, scanner, "add http://example.com/a.mp3")
	sendCommand(t, conn, scanner, "play")
	lines := sendCommand(t, conn, scanner, "status")
	first := parseResponse(lines)["songid"]
	if first == "" {
		t.Fatal("expected songid on first track, got empty")
	}

	sendCommand(t, conn, scanner, "clear")
	sendCommand(t, conn, scanner, "add http://example.com/b.mp3")
	sendCommand(t, conn, scanner, "play")
	lines = sendCommand(t, conn, scanner, "status")
	second := parseResponse(lines)["songid"]
	if second == "" {
		t.Fatal("expected songid on second track, got empty")
	}
	if second == first {
		t.Errorf("expected songid to change after Clear+Add, both = %s", first)
	}
}

// TDD: Server state is queryable via Go API for test assertions
func TestServer_GoAPIState(t *testing.T) {
	srv := New()
	defer srv.Close()

	conn, scanner := connect(t, srv.Addr())
	defer conn.Close()

	sendCommand(t, conn, scanner, "add http://example.com/track.mp3")
	sendCommand(t, conn, scanner, "play")
	sendCommand(t, conn, scanner, "setvol 75")

	// Query state via Go API (not via MPD protocol)
	state := srv.State()
	if state.PlayState != "play" {
		t.Errorf("Expected Go API state 'play', got '%s'", state.PlayState)
	}
	if state.CurrentFile != "http://example.com/track.mp3" {
		t.Errorf("Expected Go API file, got '%s'", state.CurrentFile)
	}
	if state.Volume != 75 {
		t.Errorf("Expected Go API volume 75, got %d", state.Volume)
	}
}
