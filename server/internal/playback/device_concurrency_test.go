package playback

import (
	"errors"
	"sync"
	"testing"
	"time"

	"audiod/internal/mpd"
)

// recordingMPDClient appends every command it receives so a test can check
// that a multi-command sequence wasn't interleaved with another caller's.
type recordingMPDClient struct {
	mu       sync.Mutex
	commands []string
	delay    time.Duration
	blockAll chan struct{}
}

func (c *recordingMPDClient) record(cmd string) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.mu.Lock()
	c.commands = append(c.commands, cmd)
	c.mu.Unlock()
}

func (c *recordingMPDClient) sequence() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.commands...)
}

func (c *recordingMPDClient) Play() error          { c.record("play"); return nil }
func (c *recordingMPDClient) Pause() error         { c.record("pause"); return nil }
func (c *recordingMPDClient) Stop() error          { c.record("stop"); return nil }
func (c *recordingMPDClient) Seek(int) error       { c.record("seek"); return nil }
func (c *recordingMPDClient) LoadURL(string) error { c.record("load"); return nil }
func (c *recordingMPDClient) Close() error         { return nil }

func (c *recordingMPDClient) SetVolume(int) error {
	c.record("setvol")
	return errors.New("ACK [52@0] {setvol} problems setting volume: No mixer")
}

func (c *recordingMPDClient) Status() (mpd.Status, error) {
	if c.blockAll != nil {
		<-c.blockAll
	}
	c.record("status")
	return mpd.Status{State: "play"}, nil
}

func (c *recordingMPDClient) CurrentSong() (mpd.CurrentSong, error) {
	return mpd.CurrentSong{}, nil
}

// P1-7: supportsVolume is written by SetVolume and read by the session
// response builder. The device is shared through the resolver cache, so those
// happen on different goroutines with no synchronization. Run under -race.
func TestMPDPlaybackDevice_SupportsVolume_NoDataRace(t *testing.T) {
	device := NewMPDPlaybackDevice(&recordingMPDClient{})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = device.SetVolume(50)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = device.SupportsVolume()
		}()
	}
	wg.Wait()

	if device.SupportsVolume() {
		t.Error("expected SupportsVolume=false after a mixerless setvol")
	}
}

// P2-14: Play is Clear/Add + Play + Seek as separate MPD commands. Two
// near-simultaneous plays interleaved into a two-song queue that then
// auto-advanced on its own, invisibly.
func TestMPDPlaybackDevice_Play_IsNotInterleaved(t *testing.T) {
	client := &recordingMPDClient{delay: time.Millisecond}
	device := NewMPDPlaybackDevice(client)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := device.Play("http://x/stream", 30); err != nil {
				t.Errorf("Play: %v", err)
			}
		}()
	}
	wg.Wait()

	got := client.sequence()
	want := []string{"load", "play", "seek", "load", "play", "seek"}
	if len(got) != len(want) {
		t.Fatalf("command sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("interleaved play commands: got %v, want %v", got, want)
		}
	}
}

// P1-8: gompd sets no read deadline, so a host that vanishes with the socket
// open parks a command forever. The device must give up and report the box as
// unreachable rather than pinning the caller (and the user's session lock).
func TestMPDPlaybackDevice_CommandTimesOut(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	client := &recordingMPDClient{blockAll: block}

	device := NewMPDPlaybackDevice(client)
	device.timeout = 30 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		_, err := device.Status()
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrDeviceUnreachable) {
			t.Errorf("want ErrDeviceUnreachable, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Status never returned — a vanished MPD host pins the caller forever")
	}

	// Once a command has timed out the connection is unusable; further
	// commands must fail fast instead of queueing behind the stuck one.
	if _, err := device.Status(); !errors.Is(err, ErrDeviceUnreachable) {
		t.Errorf("second command: want fast ErrDeviceUnreachable, got %v", err)
	}
}
