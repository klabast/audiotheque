package playback

import (
	"testing"
	"time"
)

// withDisconnectGracePeriod overrides the package-level grace period for the
// duration of a test, restoring the previous value on cleanup. Mirrors the
// keepalive-timing override pattern used in internal/websocket/hub_test.go.
func withDisconnectGracePeriod(t *testing.T, d time.Duration) {
	t.Helper()
	prev := disconnectGracePeriod
	disconnectGracePeriod = d
	t.Cleanup(func() { disconnectGracePeriod = prev })
}

// waitUntil polls cond until it's true or the deadline passes, returning the
// final result of cond. Used to observe the async removal a grace-period
// timer performs.
func waitUntil(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return cond()
}

func TestBrowserDeviceRegistry_RegisterListUnregister(t *testing.T) {
	withDisconnectGracePeriod(t, 0)
	r := NewBrowserDeviceRegistry()

	r.Register("c1", 42, "Chrome on macOS")
	r.Register("c2", 42, "Safari on iPhone")
	r.Register("c3", 7, "Firefox on Linux")

	devs := r.ListByUser(42)
	if len(devs) != 2 {
		t.Fatalf("expected 2 devices for user 42, got %d", len(devs))
	}

	r.Unregister("c1")
	if !waitUntil(200*time.Millisecond, func() bool { return len(r.ListByUser(42)) == 1 }) {
		t.Fatalf("expected 1 device for user 42 after unregister, got %d", len(r.ListByUser(42)))
	}
	devs = r.ListByUser(42)
	if devs[0].ClientID != "c2" {
		t.Errorf("expected remaining device c2, got %q", devs[0].ClientID)
	}

	if d, ok := r.Get("c3"); !ok || d.UserID != 7 {
		t.Errorf("Get(c3): got %v, %v", d, ok)
	}
}

func TestBrowserDeviceRegistry_AnonymousExcludedFromList(t *testing.T) {
	r := NewBrowserDeviceRegistry()
	r.Register("c1", 0, "Browser") // anonymous
	r.Register("c2", 1, "Chrome")

	if got := r.ListByUser(0); len(got) != 0 {
		t.Errorf("ListByUser(0) should return no anonymous devices; got %d", len(got))
	}
	if got := r.ListByUser(1); len(got) != 1 {
		t.Errorf("ListByUser(1) expected 1 device; got %d", len(got))
	}
}

func TestBrowserDeviceRegistry_RegisterReplaces(t *testing.T) {
	r := NewBrowserDeviceRegistry()
	r.Register("c1", 42, "OldName")
	r.Register("c1", 42, "NewName")
	d, _ := r.Get("c1")
	if d.Name != "NewName" {
		t.Errorf("expected re-register to overwrite name; got %q", d.Name)
	}
}

func TestBrowserDeviceRegistry_EmptyClientIDIgnored(t *testing.T) {
	r := NewBrowserDeviceRegistry()
	r.Register("", 42, "Bogus")
	if got := r.ListByUser(42); len(got) != 0 {
		t.Errorf("empty clientID must not register; got %d", len(got))
	}
	r.Unregister("") // must not panic
}

// TestBrowserDeviceRegistry_GetResolvesDuringGraceWindow is the regression
// test for the "session dies mid-reconnect" bug: a device that just
// disconnected must still resolve (Get returns ok=true) while the grace
// period is running, so GetSession doesn't delete the session while the
// browser tab's WebSocket is merely reconnecting.
func TestBrowserDeviceRegistry_GetResolvesDuringGraceWindow(t *testing.T) {
	withDisconnectGracePeriod(t, 200*time.Millisecond)
	r := NewBrowserDeviceRegistry()

	r.Register("c1", 42, "Chrome on macOS")
	r.Unregister("c1")

	if _, ok := r.Get("c1"); !ok {
		t.Fatal("device must still resolve immediately after unregister (grace window)")
	}
}

// TestBrowserDeviceRegistry_ReconnectCancelsGraceRemoval covers the reconnect
// race: if the same clientID re-registers before the grace period elapses
// (the new WS connection came up before the old one's teardown finished),
// the pending removal must be cancelled — the device must never disappear.
func TestBrowserDeviceRegistry_ReconnectCancelsGraceRemoval(t *testing.T) {
	withDisconnectGracePeriod(t, 40*time.Millisecond)
	r := NewBrowserDeviceRegistry()

	r.Register("c1", 42, "Chrome on macOS")
	r.Unregister("c1")
	r.Register("c1", 42, "Chrome on macOS") // reconnected before the grace timer fired

	// Sleep past the ORIGINAL grace window. If the timer wasn't cancelled,
	// the reconnected device would be wrongly deleted here.
	time.Sleep(80 * time.Millisecond)

	if _, ok := r.Get("c1"); !ok {
		t.Fatal("re-register before grace expiry must cancel the pending removal")
	}
}

// P1-6: the grace timer's closure deleted unconditionally. If it had already
// fired and was waiting on the mutex when Register ran, timer.Stop() returned
// false, Register stored the fresh device, and the late closure then deleted
// it — a tab reconnecting right at the 60s mark simply vanished.
func TestBrowserDeviceRegistry_LateGraceTimerKeepsReconnectedDevice(t *testing.T) {
	withDisconnectGracePeriod(t, time.Hour) // never fires on its own
	r := NewBrowserDeviceRegistry()

	r.Register("c1", 42, "Chrome on macOS")
	r.Unregister("c1")

	r.mu.RLock()
	stale := r.pendingRemoval["c1"]
	r.mu.RUnlock()
	if stale == nil {
		t.Fatal("precondition: Unregister must arm a removal")
	}

	// The tab reconnects, then the already-fired timer finally gets the mutex.
	r.Register("c1", 42, "Chrome on macOS")
	r.expireRemoval("c1", stale)

	if _, ok := r.Get("c1"); !ok {
		t.Fatal("a grace timer from before the reconnect removed the freshly registered device")
	}
}

// TestBrowserDeviceRegistry_UnregisterExpiresWithoutReconnect is the other
// half: with no reconnect, the device must actually disappear once the grace
// period elapses — otherwise stale devices/sessions would live forever.
func TestBrowserDeviceRegistry_UnregisterExpiresWithoutReconnect(t *testing.T) {
	withDisconnectGracePeriod(t, 20*time.Millisecond)
	r := NewBrowserDeviceRegistry()

	r.Register("c1", 42, "Chrome on macOS")
	r.Unregister("c1")

	if !waitUntil(200*time.Millisecond, func() bool {
		_, ok := r.Get("c1")
		return !ok
	}) {
		t.Fatal("device should be removed once the grace period elapses without a reconnect")
	}
}
