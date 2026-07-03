package websocket

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// TestHandler_RegistersAuthenticatedUserID is the regression test for the
// multi-tab sync bug: when SetUserIDGetter is wired, upgraded connections
// must register with the resolved userID — not 0 — so per-user broadcasts
// reach them.
func TestHandler_RegistersAuthenticatedUserID(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(r *http.Request) (int64, error) {
		// Stand-in for the auth middleware: trust an "X-User" header.
		switch r.Header.Get("X-User") {
		case "alice":
			return 42, nil
		case "":
			return 0, errors.New("unauthorized")
		default:
			return 0, errors.New("unknown user")
		}
	})

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	header := http.Header{"X-User": []string{"alice"}}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Wait for the hub to process the register.
	if !waitFor(t, 200*time.Millisecond, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("client never registered")
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for c := range hub.clients {
		assert.Equal(t, int64(42), c.UserID, "client must register under the authenticated userID")
		assert.NotEmpty(t, c.ID, "client should have a non-empty hub-assigned ID")
		return
	}
	t.Fatal("no client registered")
}

// TestHandler_RejectsUpgradeWhenAuthFails refuses upgrades that aren't
// authenticated. Letting them through registers a userID=0 client which still
// receives every Hub.Broadcast and can submit incoming messages — better to
// fail closed.
func TestHandler_RejectsUpgradeWhenAuthFails(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(*http.Request) (int64, error) {
		return 0, errors.New("unauthorized")
	})

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected dial to fail when auth errors")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount(), "no client should register on auth failure")
}

// TestHandler_RejectsUpgradeWhenGetterUnset refuses upgrades when the handler
// hasn't been wired with a getter at all — fail closed rather than admit
// anonymous connections.
func TestHandler_RejectsUpgradeWhenGetterUnset(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	handler := NewHandler(hub) // no getter wired

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()

	wsURL, _ := url.Parse("ws" + strings.TrimPrefix(srv.URL, "http"))
	_, resp, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err == nil {
		t.Fatal("expected dial to fail when no getter wired")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// TestHub_LifecycleHandlersFire verifies the OnRegister/OnUnregister
// callbacks are invoked with the right Client. The browser-tab handoff
// machinery depends on these to maintain the ephemeral device registry.
func TestHub_LifecycleHandlersFire(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	var registered, unregistered []*Client
	var mu testingMutex
	hub.SetLifecycleHandlers(
		func(c *Client) { mu.Lock(); registered = append(registered, c); mu.Unlock() },
		func(c *Client) { mu.Lock(); unregistered = append(unregistered, c); mu.Unlock() },
	)

	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(_ *http.Request) (int64, error) { return 9, nil })

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	header := http.Header{"User-Agent": []string{"Mozilla/5.0 (Macintosh) Chrome/120"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	if !waitFor(t, 200*time.Millisecond, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(registered) == 1
	}) {
		t.Fatal("OnRegister never fired")
	}
	mu.Lock()
	if registered[0].UserID != 9 {
		t.Errorf("UserID = %d; want 9", registered[0].UserID)
	}
	if registered[0].UserAgent == "" {
		t.Error("UserAgent should be captured on upgrade")
	}
	mu.Unlock()

	conn.Close()

	if !waitFor(t, 500*time.Millisecond, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(unregistered) == 1
	}) {
		t.Fatal("OnUnregister never fired")
	}
}

// TestHub_SendToClient targets a single client. Used to deliver per-tab
// messages like the client-id welcome.
func TestHub_SendToClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(_ *http.Request) (int64, error) { return 1, nil })

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if !waitFor(t, 200*time.Millisecond, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("client never registered")
	}

	var clientID string
	hub.mu.RLock()
	for c := range hub.clients {
		clientID = c.ID
	}
	hub.mu.RUnlock()
	if clientID == "" {
		t.Fatal("no client ID")
	}

	if err := hub.SendToClient(clientID, Message{Type: "client-id", Data: map[string]string{"clientId": clientID}}); err != nil {
		t.Fatalf("SendToClient: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(payload), `"type":"client-id"`) {
		t.Fatalf("unexpected payload: %s", string(payload))
	}

	// Sending to an unknown client is a no-op — must not error.
	if err := hub.SendToClient("nope", Message{Type: "x"}); err != nil {
		t.Errorf("SendToClient(unknown) should be no-op; got %v", err)
	}
}

// testingMutex is a tiny wrapper around sync.Mutex to keep the test code
// readable when assertions need locking around shared state.
type testingMutex struct{ m sync.Mutex }

func (t *testingMutex) Lock()   { t.m.Lock() }
func (t *testingMutex) Unlock() { t.m.Unlock() }

// TestServeWs_UsesClientSuppliedUUID is the regression test for the
// LAN→WLAN device-ID bug: a browser tab that sends a stable, sessionStorage-
// persisted UUID as ?clientId= must be assigned that ID (prefixed) rather
// than a fresh hub-counter value, so the deviceID a session is bound to
// survives a network-change reconnect.
func TestServeWs_UsesClientSuppliedUUID(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(*http.Request) (int64, error) { return 1, nil })

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()

	const uuid = "11111111-2222-3333-4444-555555555555"
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/?clientId=" + uuid

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if !waitFor(t, 200*time.Millisecond, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("client never registered")
	}

	hub.mu.RLock()
	var gotID string
	for c := range hub.clients {
		gotID = c.ID
	}
	hub.mu.RUnlock()

	want := "b-" + uuid
	if gotID != want {
		t.Errorf("client ID = %q, want %q", gotID, want)
	}
}

// TestServeWs_FallsBackToCounterForInvalidClientID covers absent/malformed
// clientId query params (older client build, curl, a tampered value) — the
// hub must fall back to its own monotonic "c<n>" assignment rather than
// trust an arbitrary string as a device identity.
func TestServeWs_FallsBackToCounterForInvalidClientID(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(*http.Request) (int64, error) { return 1, nil })

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/?clientId=not-a-uuid"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if !waitFor(t, 200*time.Millisecond, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("client never registered")
	}

	hub.mu.RLock()
	var gotID string
	for c := range hub.clients {
		gotID = c.ID
	}
	hub.mu.RUnlock()

	if !strings.HasPrefix(gotID, "c") {
		t.Errorf("expected fallback counter-style ID (c<n>), got %q", gotID)
	}
}

// TestHub_ReconnectWithSameClientIDEvictsStaleConnection is the targeted
// regression test for the reconnect race: a new connection registering with
// the same client-supplied ID as a still-technically-open old connection
// (its dead socket hasn't been noticed yet) must evict the old one — not
// coexist with it. Coexistence let SendToClient(id) (used for the client-id
// welcome) pick the stale, dying connection, so the reconnecting tab never
// learned its own ID.
func TestHub_ReconnectWithSameClientIDEvictsStaleConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(*http.Request) (int64, error) { return 1, nil })

	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer srv.Close()

	const uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/?clientId=" + uuid

	connA, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	defer connA.Close()

	if !waitFor(t, 200*time.Millisecond, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("client A never registered")
	}

	// Reconnect with the SAME clientId before A's connection is torn down —
	// simulates the new tab's WS coming up before the hub notices the old
	// socket is dead (LAN→WLAN handoff).
	connB, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial B: %v", err)
	}
	defer connB.Close()

	if !waitFor(t, 200*time.Millisecond, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatalf("expected stale connection to be evicted, got %d clients", hub.ClientCount())
	}

	// The server must have actively closed connA — it's the stale one.
	if err := connA.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if _, _, err := connA.ReadMessage(); err == nil {
		t.Fatal("expected the stale connection A to have been closed by the server")
	}

	// connB must still be alive and own the ID.
	hub.mu.RLock()
	var gotID string
	for c := range hub.clients {
		gotID = c.ID
	}
	hub.mu.RUnlock()
	if want := "b-" + uuid; gotID != want {
		t.Errorf("surviving client ID = %q, want %q", gotID, want)
	}
}
