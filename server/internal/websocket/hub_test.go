package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

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
