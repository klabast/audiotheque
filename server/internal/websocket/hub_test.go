package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()
	assert.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.broadcast)
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_ClientRegistration(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a mock client
	client := &Client{
		hub:  hub,
		conn: nil, // Mock, not using actual connection
		send: make(chan []byte, 256),
	}

	// Register client
	hub.register <- client
	time.Sleep(10 * time.Millisecond) // Give hub time to process

	assert.Equal(t, 1, hub.ClientCount())

	// Unregister client
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond) // Give hub time to process

	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock clients
	client1 := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, 256),
	}
	client2 := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, 256),
	}

	// Register clients
	hub.register <- client1
	hub.register <- client2
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 2, hub.ClientCount())

	// Broadcast a message
	msg := Message{
		Type: "test",
		Data: map[string]string{"hello": "world"},
	}
	err := hub.Broadcast(msg)
	assert.NoError(t, err)

	// Verify both clients received the message
	select {
	case data := <-client1.send:
		var received Message
		err := json.Unmarshal(data, &received)
		assert.NoError(t, err)
		assert.Equal(t, "test", received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client 1 did not receive message")
	}

	select {
	case data := <-client2.send:
		var received Message
		err := json.Unmarshal(data, &received)
		assert.NoError(t, err)
		assert.Equal(t, "test", received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client 2 did not receive message")
	}
}

func TestHub_BroadcastWithNoClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Broadcast without any clients connected
	msg := Message{
		Type: "test",
		Data: "no clients",
	}
	err := hub.Broadcast(msg)
	assert.NoError(t, err)

	// Should not panic or error
	time.Sleep(10 * time.Millisecond)
}

func TestHub_ClientCount(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	assert.Equal(t, 0, hub.ClientCount())

	// Register 3 clients
	clients := make([]*Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = &Client{
			hub:  hub,
			conn: nil,
			send: make(chan []byte, 256),
		}
		hub.register <- clients[i]
	}
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 3, hub.ClientCount())

	// Unregister 1 client
	hub.unregister <- clients[0]
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 2, hub.ClientCount())
}

func TestHub_BroadcastToUserExcept(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Two browser tabs of the same user.
	tabA := &Client{
		hub:    hub,
		send:   make(chan []byte, 256),
		ID:     "c1",
		UserID: 7,
	}
	tabB := &Client{
		hub:    hub,
		send:   make(chan []byte, 256),
		ID:     "c2",
		UserID: 7,
	}
	hub.register <- tabA
	hub.register <- tabB
	time.Sleep(10 * time.Millisecond)

	msg := Message{Type: "playback-session", Data: "hi"}
	if err := hub.BroadcastToUserExcept(7, "c1", msg); err != nil {
		t.Fatalf("BroadcastToUserExcept: %v", err)
	}

	// tabA (the sender) must NOT receive the message.
	select {
	case <-tabA.send:
		t.Fatal("sender tab A should have been excluded")
	case <-time.After(50 * time.Millisecond):
	}

	// tabB must receive it.
	select {
	case data := <-tabB.send:
		var got Message
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Type != "playback-session" {
			t.Errorf("type = %q", got.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("tab B did not receive message")
	}
}

// TestHub_ConcurrentBroadcastDeletionRace stresses the broadcast path. The
// previous implementation deleted from h.clients while only holding RLock,
// which races with BroadcastToUser / SendToClient readers. Run with -race;
// without the fix this trips "concurrent map iteration and map write".
func TestHub_ConcurrentBroadcastDeletionRace(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	const N = 50
	for i := 0; i < N; i++ {
		c := &Client{
			hub:    hub,
			send:   make(chan []byte, 1), // tiny buffer → broadcasts overflow → delete path
			ID:     "",
			UserID: int64(i),
		}
		hub.register <- c
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && hub.ClientCount() != N {
		time.Sleep(2 * time.Millisecond)
	}
	if hub.ClientCount() != N {
		t.Fatalf("expected %d clients, got %d", N, hub.ClientCount())
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = hub.Broadcast(Message{Type: "x", Data: "y"})
			}
		}
	}()

	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = hub.BroadcastToUser(int64(r), Message{Type: "u"})
					_ = hub.ClientCount()
				}
			}
		}(r)
	}

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// withKeepaliveTimings overrides the package keepalive knobs for the
// duration of a test. Production values are seconds; tests need them in
// tens of milliseconds so the suite stays fast. The previous values are
// restored on test cleanup; the knobs are atomic so background goroutines
// from earlier tests can read them safely while we swap.
func withKeepaliveTimings(t *testing.T, ping, pong, write time.Duration) {
	t.Helper()
	prevPing, prevPong, prevWrite := pingInterval.Load(), pongWait.Load(), writeWait.Load()
	pingInterval.Store(int64(ping))
	pongWait.Store(int64(pong))
	writeWait.Store(int64(write))
	t.Cleanup(func() {
		pingInterval.Store(prevPing)
		pongWait.Store(prevPong)
		writeWait.Store(prevWrite)
	})
}

// dialWithUserGetter wires a handler with a fixed userID and returns a
// connected client.
func dialWithUserGetter(t *testing.T, hub *Hub, userID int64) *websocket.Conn {
	t.Helper()
	handler := NewHandler(hub)
	handler.SetUserIDGetter(func(_ *http.Request) (int64, error) { return userID, nil })
	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	t.Cleanup(srv.Close)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// TestWebSocket_ServerSendsPings is the keepalive contract: an idle server
// must send WS ping control frames on a regular cadence so intermediaries
// (proxies, NAT, browser idle timers) don't silently drop the connection.
//
// Without keepalive, production was losing connections every ~90s. The new
// tab that reconnected got a fresh clientID while the persisted session
// still named the dead clientID — surfaced in the UI as a phantom "Remote
// device" that pauses local playback.
func TestWebSocket_ServerSendsPings(t *testing.T) {
	withKeepaliveTimings(t, 30*time.Millisecond, 100*time.Millisecond, 100*time.Millisecond)

	hub := NewHub()
	go hub.Run()
	conn := dialWithUserGetter(t, hub, 1)

	pings := make(chan struct{}, 8)
	conn.SetPingHandler(func(appData string) error {
		select {
		case pings <- struct{}{}:
		default:
		}
		// Mirror the gorilla default so the server doesn't think we're dead.
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(50*time.Millisecond))
	})

	// Ping/pong handlers fire from inside ReadMessage, so we have to keep
	// reading. We don't expect any data frames here.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Two pings inside ~150ms is the contract — proves the cadence, not just
	// that a single frame happened to arrive.
	for i := 0; i < 2; i++ {
		select {
		case <-pings:
		case <-time.After(150 * time.Millisecond):
			t.Fatalf("only received %d ping(s); server is not pinging on a cadence", i)
		}
	}

	conn.Close()
	<-done
}

// TestWebSocket_DeadClientEvictedAfterPongTimeout is the other half of the
// contract: if a client stops responding (silent TCP, dead intermediary, OS
// dropped the socket without closing) the server must drop it within a
// bounded time. Otherwise the orphan-session machinery on the next reconnect
// is racing a hub that still believes the old client is live.
func TestWebSocket_DeadClientEvictedAfterPongTimeout(t *testing.T) {
	withKeepaliveTimings(t, 20*time.Millisecond, 60*time.Millisecond, 60*time.Millisecond)

	hub := NewHub()
	go hub.Run()
	conn := dialWithUserGetter(t, hub, 1)

	// Wait for register.
	if !waitFor(t, 200*time.Millisecond, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("client never registered")
	}

	// Black-hole pings: receive them but never pong back. Drain reads so the
	// handler can fire — without a read, gorilla never invokes SetPingHandler.
	conn.SetPingHandler(func(string) error { return nil })
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// pongWait=60ms; allow a couple of cycles plus scheduling slack. If the
	// server has no read deadline at all (current state), the count stays at
	// 1 forever and this fails.
	if !waitFor(t, 500*time.Millisecond, func() bool { return hub.ClientCount() == 0 }) {
		t.Fatalf("dead client was not evicted; ClientCount=%d", hub.ClientCount())
	}
}

// TestWebSocket_LiveClientNotEvicted is the safety net: a well-behaved
// client (the gorilla default ping handler auto-pongs) must NOT be evicted
// by the keepalive. Without this, a too-aggressive deadline would just
// invert the original bug.
func TestWebSocket_LiveClientNotEvicted(t *testing.T) {
	withKeepaliveTimings(t, 20*time.Millisecond, 80*time.Millisecond, 80*time.Millisecond)

	hub := NewHub()
	go hub.Run()
	conn := dialWithUserGetter(t, hub, 1)

	// Default gorilla ping handler auto-pongs; the read loop just has to
	// run so handlers fire. A blocking ReadMessage is sufficient — it will
	// only return when the test cleanup closes the conn.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Survive several ping cycles.
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount(), "responsive client must not be evicted")
}

func TestHub_BroadcastToUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create clients for two different users
	user1Client := &Client{
		hub:    hub,
		conn:   nil,
		send:   make(chan []byte, 256),
		UserID: 1,
	}
	user2Client := &Client{
		hub:    hub,
		conn:   nil,
		send:   make(chan []byte, 256),
		UserID: 2,
	}

	hub.register <- user1Client
	hub.register <- user2Client
	time.Sleep(10 * time.Millisecond)

	// Broadcast to user 1 only
	msg := Message{Type: "playback-update", Data: "user1 data"}
	err := hub.BroadcastToUser(1, msg)
	assert.NoError(t, err)

	// User 1 should receive the message
	select {
	case data := <-user1Client.send:
		var received Message
		err := json.Unmarshal(data, &received)
		assert.NoError(t, err)
		assert.Equal(t, "playback-update", received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("User 1 client did not receive message")
	}

	// User 2 should NOT receive the message
	select {
	case <-user2Client.send:
		t.Fatal("User 2 client should not have received the message")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message for user 2
	}
}
