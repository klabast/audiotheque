package playback

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"audiod/internal/mpd"
)

// fakeMPDClient is a mpd.Client whose Status can be made to fail (simulating a
// dropped connection) or to block forever (simulating a host that vanished
// with the TCP connection still established).
type fakeMPDClient struct {
	mu         sync.Mutex
	closed     bool
	statusErr  error
	blockUntil chan struct{}
}

func (c *fakeMPDClient) Play() error         { return nil }
func (c *fakeMPDClient) Pause() error        { return nil }
func (c *fakeMPDClient) Stop() error         { return nil }
func (c *fakeMPDClient) SetVolume(int) error { return nil }
func (c *fakeMPDClient) Seek(int) error      { return nil }
func (c *fakeMPDClient) LoadURL(string) error {
	if c.blockUntil != nil {
		<-c.blockUntil
	}
	return nil
}

func (c *fakeMPDClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeMPDClient) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *fakeMPDClient) Status() (mpd.Status, error) {
	c.mu.Lock()
	err := c.statusErr
	block := c.blockUntil
	c.mu.Unlock()
	if block != nil {
		<-block
	}
	if err != nil {
		return mpd.Status{}, err
	}
	return mpd.Status{State: "play"}, nil
}

func (c *fakeMPDClient) CurrentSong() (mpd.CurrentSong, error) {
	return mpd.CurrentSong{}, nil
}

func (c *fakeMPDClient) setStatusErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.statusErr = err
}

func (c *fakeMPDClient) setBlock(ch chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blockUntil = ch
}

func newTestResolver(t *testing.T, devices ...Device) (*RegistryDeviceResolver, *[]*fakeMPDClient) {
	t.Helper()
	registry := NewInMemoryDeviceRegistry()
	for _, d := range devices {
		if err := registry.Register(d); err != nil {
			t.Fatalf("register %s: %v", d.ID, err)
		}
	}
	resolver := NewRegistryDeviceResolver(registry, slog.Default())
	var dialed []*fakeMPDClient
	var mu sync.Mutex
	resolver.dial = func(string) (mpd.Client, error) {
		c := &fakeMPDClient{}
		mu.Lock()
		dialed = append(dialed, c)
		mu.Unlock()
		return c, nil
	}
	return resolver, &dialed
}

// P1-9: a stale connection was dropped from the map without closing it, so
// every reconnect leaked a socket and an MPD client slot.
func TestRegistryDeviceResolver_ClosesStaleConnection(t *testing.T) {
	resolver, dialed := newTestResolver(t, Device{
		ID: "mpd-1", Type: DeviceTypeMPD, Address: "127.0.0.1:6600", UserID: 1,
	})

	if _, err := resolver.ResolveDevice("mpd-1"); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if len(*dialed) != 1 {
		t.Fatalf("expected 1 dial, got %d", len(*dialed))
	}
	first := (*dialed)[0]

	// Connection goes stale; the next resolve must drop and reconnect.
	first.setStatusErr(errors.New("broken pipe"))

	if _, err := resolver.ResolveDevice("mpd-1"); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if len(*dialed) != 2 {
		t.Fatalf("expected a reconnect, got %d dials", len(*dialed))
	}
	if !first.isClosed() {
		t.Error("stale MPD connection was dropped without Close — the socket leaks")
	}
}

// P0-1: the resolver must distinguish "this device does not exist" from "we
// could not reach it". GetSession deletes the session only on the former.
func TestRegistryDeviceResolver_ErrorsAreDistinguishable(t *testing.T) {
	registry := NewInMemoryDeviceRegistry()
	if err := registry.Register(Device{
		ID: "mpd-1", Type: DeviceTypeMPD, Address: "127.0.0.1:6600", UserID: 1,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolver := NewRegistryDeviceResolver(registry, slog.Default())
	resolver.dial = func(string) (mpd.Client, error) {
		return nil, errors.New("dial tcp 127.0.0.1:6600: connect: connection refused")
	}

	_, err := resolver.ResolveDevice("nope")
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("unknown device: want ErrDeviceNotFound, got %v", err)
	}

	_, err = resolver.ResolveDevice("mpd-1")
	if !errors.Is(err, ErrDeviceUnreachable) {
		t.Errorf("refused dial: want ErrDeviceUnreachable, got %v", err)
	}
	if errors.Is(err, ErrDeviceNotFound) {
		t.Error("a refused dial must never read as device-not-found — it destroys the session")
	}
}

// P1-8: the resolver held one global mutex across MPD network I/O. With gompd
// setting no deadlines, a host that vanished mid-connection parked Status()
// forever and every other device — and therefore every other user's playback
// endpoint — wedged behind that lock.
func TestRegistryDeviceResolver_WedgedDeviceDoesNotBlockOthers(t *testing.T) {
	registry := NewInMemoryDeviceRegistry()
	for _, d := range []Device{
		{ID: "mpd-wedged", Type: DeviceTypeMPD, Address: "10.0.0.1:6600", UserID: 1},
		{ID: "mpd-healthy", Type: DeviceTypeMPD, Address: "10.0.0.2:6600", UserID: 1},
	} {
		if err := registry.Register(d); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	resolver := NewRegistryDeviceResolver(registry, slog.Default())

	var wedgedClient *fakeMPDClient
	resolver.dial = func(addr string) (mpd.Client, error) {
		c := &fakeMPDClient{}
		if addr == "10.0.0.1:6600" {
			wedgedClient = c
		}
		return c, nil
	}

	// Warm both connections while everything still answers.
	if _, err := resolver.ResolveDevice("mpd-healthy"); err != nil {
		t.Fatalf("resolve healthy: %v", err)
	}
	if _, err := resolver.ResolveDevice("mpd-wedged"); err != nil {
		t.Fatalf("resolve wedged: %v", err)
	}

	// The wedged host vanishes with the socket still open: the cached-connection
	// health check parks forever. This is the poller's code path.
	unblock := make(chan struct{})
	defer close(unblock)
	wedgedClient.setBlock(unblock)

	parked := make(chan struct{})
	go func() {
		close(parked)
		_, _ = resolver.ResolveDevice("mpd-wedged")
	}()
	<-parked
	time.Sleep(20 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		_, err := resolver.ResolveDevice("mpd-healthy")
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("resolve healthy while another device is wedged: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a wedged device blocked resolution of an unrelated device — one global lock across network I/O")
	}
}
