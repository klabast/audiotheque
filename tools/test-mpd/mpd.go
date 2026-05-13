package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MPDServer is a standalone fake MPD server that speaks the MPD protocol
// over TCP and tracks all received commands for test observation.
type MPDServer struct {
	mu        sync.Mutex
	state     string // "play", "pause", "stop"
	volume    int
	elapsed   int
	playlist  []string
	songIDs   []int // parallel to playlist; bumped per Add like real MPD
	nextID    int
	history   []CommandRecord
	mixerless bool // true → setvol returns the no-mixer ACK
}

// CommandRecord is a single command received by the MPD server.
type CommandRecord struct {
	Command   string `json:"command"`
	Args      string `json:"args,omitempty"`
	Timestamp string `json:"timestamp"`
}

// StateResponse is the JSON response for GET /state.
type StateResponse struct {
	PlayState   string   `json:"playState"`
	CurrentFile string   `json:"currentFile"`
	Elapsed     int      `json:"elapsed"`
	Volume      int      `json:"volume"`
	Playlist    []string `json:"playlist"`
}

// NewMPDServer creates a new MPD server with default state. Spawns a 1Hz
// goroutine that increments elapsed while state=play — mirrors real MPD's
// internal clock so the position-poller has something to mirror back to
// the session, and so e2e scenarios can assert progress reporting without
// having to drive elapsed by hand.
func NewMPDServer() *MPDServer {
	s := &MPDServer{
		state:  "stop",
		volume: 100,
		nextID: 1,
	}
	go s.clock()
	return s
}

// clock advances elapsed by 1 each second while state=play. Stops counting
// when there's no playlist or state is not play. Unbounded — real MPD
// caps via track duration, but we don't track duration in the fake.
func (s *MPDServer) clock() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		if s.state == "play" && len(s.playlist) > 0 {
			s.elapsed++
		}
		s.mu.Unlock()
	}
}

// SetMixerless toggles HiFiBerry-style "no mixer" mode where setvol returns
// the same ACK string real MPD returns when the audio output has
// `mixer_type "none"`. Used by E2E to validate audiod's tolerance of
// mixerless devices.
func (s *MPDServer) SetMixerless(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mixerless = on
}

// State returns a snapshot of the server's current state.
func (s *MPDServer) State() StateResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentFile := ""
	if len(s.playlist) > 0 {
		currentFile = s.playlist[0]
	}

	playlist := make([]string, len(s.playlist))
	copy(playlist, s.playlist)

	return StateResponse{
		PlayState:   s.state,
		CurrentFile: currentFile,
		Elapsed:     s.elapsed,
		Volume:      s.volume,
		Playlist:    playlist,
	}
}

// History returns all commands received since last reset.
func (s *MPDServer) History() []CommandRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]CommandRecord, len(s.history))
	copy(out, s.history)
	return out
}

// Reset clears state and command history.
func (s *MPDServer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = "stop"
	s.volume = 100
	s.elapsed = 0
	s.playlist = nil
	s.songIDs = nil
	s.nextID = 1
	s.history = nil
	s.mixerless = false
}

// EndCurrentTrack simulates the current track finishing — real MPD reports
// state="stop" once it runs off the end of the loaded queue. The audiod
// position poller observes this transition and auto-advances the session.
// Used by E2E to drive the auto-advance path deterministically.
func (s *MPDServer) EndCurrentTrack() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = "stop"
	s.elapsed = 0
}

// ListenAndServe starts the MPD TCP server.
func (s *MPDServer) ListenAndServe(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *MPDServer) handleConn(conn net.Conn) {
	defer conn.Close()

	log.Printf("MPD client connected from %s", conn.RemoteAddr())

	// MPD greeting
	fmt.Fprintf(conn, "OK MPD 0.23.0\n")

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		response := s.handleCommand(line)
		fmt.Fprint(conn, response)
	}
}

func (s *MPDServer) handleCommand(line string) string {
	parts := strings.SplitN(line, " ", 2)
	cmd := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Record command
	if args != "" {
		log.Printf("MPD command: %s %s", cmd, args)
	} else {
		log.Printf("MPD command: %s", cmd)
	}
	s.history = append(s.history, CommandRecord{
		Command:   cmd,
		Args:      args,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})

	switch cmd {
	case "status":
		return s.cmdStatus()
	case "currentsong":
		return s.cmdCurrentSong()
	case "play":
		return s.cmdPlay()
	case "pause":
		return s.cmdPause(args)
	case "stop":
		return s.cmdStop()
	case "add":
		return s.cmdAdd(args)
	case "clear":
		return s.cmdClear()
	case "setvol":
		return s.cmdSetVol(args)
	case "seekcur":
		return s.cmdSeekCur(args)
	case "close":
		return ""
	case "ping":
		return "OK\n"
	default:
		return fmt.Sprintf("ACK [5@0] {} unknown command \"%s\"\n", cmd)
	}
}

func (s *MPDServer) cmdStatus() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("volume: %d\n", s.volume))
	b.WriteString(fmt.Sprintf("state: %s\n", s.state))
	if s.state != "stop" && len(s.playlist) > 0 {
		b.WriteString(fmt.Sprintf("elapsed: %d\n", s.elapsed))
		b.WriteString("song: 0\n")
		b.WriteString(fmt.Sprintf("songid: %d\n", s.songIDs[0]))
	}
	b.WriteString("OK\n")
	return b.String()
}

func (s *MPDServer) cmdCurrentSong() string {
	if len(s.playlist) == 0 {
		return "OK\n"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("file: %s\n", s.playlist[0]))
	b.WriteString("Pos: 0\n")
	b.WriteString(fmt.Sprintf("Id: %d\n", s.songIDs[0]))
	b.WriteString("OK\n")
	return b.String()
}

func (s *MPDServer) cmdPlay() string {
	if len(s.playlist) > 0 {
		s.state = "play"
	}
	return "OK\n"
}

func (s *MPDServer) cmdPause(args string) string {
	if args == "1" {
		s.state = "pause"
	} else if args == "0" {
		if len(s.playlist) > 0 {
			s.state = "play"
		}
	} else {
		if s.state == "play" {
			s.state = "pause"
		} else if s.state == "pause" {
			s.state = "play"
		}
	}
	return "OK\n"
}

func (s *MPDServer) cmdStop() string {
	s.state = "stop"
	s.elapsed = 0
	return "OK\n"
}

func (s *MPDServer) cmdAdd(uri string) string {
	uri = strings.Trim(uri, "\"")
	s.playlist = append(s.playlist, uri)
	s.songIDs = append(s.songIDs, s.nextID)
	s.nextID++
	return "OK\n"
}

func (s *MPDServer) cmdClear() string {
	s.playlist = nil
	s.songIDs = nil
	s.state = "stop"
	s.elapsed = 0
	return "OK\n"
}

func (s *MPDServer) cmdSetVol(args string) string {
	if s.mixerless {
		return "ACK [52@0] {setvol} problems setting volume: No mixer\n"
	}
	vol, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		return "ACK [2@0] {} invalid volume\n"
	}
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	s.volume = vol
	return "OK\n"
}

func (s *MPDServer) cmdSeekCur(args string) string {
	pos, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		return "ACK [2@0] {} invalid position\n"
	}
	s.elapsed = pos
	return "OK\n"
}
