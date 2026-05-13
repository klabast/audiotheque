package testserver

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
)

// ServerState represents the queryable internal state of the fake MPD server.
// Used by tests to assert what the fake MPD has received/done.
type ServerState struct {
	PlayState   string // "play", "pause", "stop"
	CurrentFile string
	Elapsed     int
	Volume      int
	Playlist    []string
}

// Server is a fake MPD TCP server for testing.
// It speaks a subset of the MPD protocol sufficient for audiod's needs.
type Server struct {
	listener net.Listener
	mu       sync.Mutex
	state    string // "play", "pause", "stop"
	volume   int
	elapsed  int
	playlist []string
	songIDs  []int // parallel to playlist; bumped per Add like real MPD
	nextID   int   // next songid to assign
	// mixerless mimics MPD with `mixer_type "none"` (e.g. HiFiBerry default):
	// setvol returns the same ACK string real MPD returns in that mode.
	mixerless bool
	closed    bool
}

// New creates and starts a fake MPD server on a random port.
func New() *Server {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("testserver: failed to listen: %v", err))
	}

	s := &Server{
		listener: listener,
		state:    "stop",
		volume:   100,
		nextID:   1,
	}

	go s.acceptLoop()
	return s
}

// Addr returns the address the server is listening on (e.g., "127.0.0.1:12345").
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// State returns a snapshot of the server's internal state for test assertions.
func (s *Server) State() ServerState {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentFile := ""
	if len(s.playlist) > 0 {
		currentFile = s.playlist[0]
	}

	return ServerState{
		PlayState:   s.state,
		CurrentFile: currentFile,
		Elapsed:     s.elapsed,
		Volume:      s.volume,
		Playlist:    append([]string{}, s.playlist...),
	}
}

// SetMixerless toggles mixerless mode (setvol returns the same ACK string
// real MPD returns when the audio output has `mixer_type "none"`).
func (s *Server) SetMixerless(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mixerless = on
}

// SetState forces the playback state without going through the protocol.
// Lets integration tests simulate device-side conditions (e.g. transient
// "stop" while a freshly-loaded URL is buffering) that are hard to reach
// via the MPD command set alone.
func (s *Server) SetState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// SetElapsed sets the current track's elapsed seconds; integration tests use
// this to simulate "track has been playing for N seconds" without sleeping.
func (s *Server) SetElapsed(elapsed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.elapsed = elapsed
}

// Close shuts down the fake MPD server.
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.listener.Close()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			log.Printf("testserver: accept error: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	// Send MPD greeting
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

func (s *Server) handleCommand(line string) string {
	parts := strings.SplitN(line, " ", 2)
	cmd := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *Server) cmdStatus() string {
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

func (s *Server) cmdCurrentSong() string {
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

func (s *Server) cmdPlay() string {
	if len(s.playlist) > 0 {
		s.state = "play"
	}
	return "OK\n"
}

func (s *Server) cmdPause(args string) string {
	if args == "1" {
		s.state = "pause"
	} else if args == "0" {
		if len(s.playlist) > 0 {
			s.state = "play"
		}
	} else {
		// Toggle
		if s.state == "play" {
			s.state = "pause"
		} else if s.state == "pause" {
			s.state = "play"
		}
	}
	return "OK\n"
}

func (s *Server) cmdStop() string {
	s.state = "stop"
	s.elapsed = 0
	return "OK\n"
}

func (s *Server) cmdAdd(uri string) string {
	// Strip quotes (gompd sends: add "http://...")
	uri = strings.Trim(uri, "\"")
	s.playlist = append(s.playlist, uri)
	s.songIDs = append(s.songIDs, s.nextID)
	s.nextID++
	return "OK\n"
}

func (s *Server) cmdClear() string {
	s.playlist = nil
	s.songIDs = nil
	s.state = "stop"
	s.elapsed = 0
	return "OK\n"
}

func (s *Server) cmdSetVol(args string) string {
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

func (s *Server) cmdSeekCur(args string) string {
	pos, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		return "ACK [2@0] {} invalid position\n"
	}
	s.elapsed = pos
	return "OK\n"
}
