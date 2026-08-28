package websocket

import (
	"errors"
	"testing"
	"time"
)

// TestBroadcast_DoesNotEvictSlowClient covers the kick/reconnect loop.
//
// A single full send buffer was treated as terminal: the client was removed
// from the hub and its channel closed, i.e. disconnected. A phone on a weak
// link or a backgrounded tab whose write pump the browser throttles would fill
// 256 slots during a scan, get evicted, reconnect, and be evicted again for the
// scan's duration. A slow reader should miss messages, not lose its connection.
func TestBroadcast_DoesNotEvictSlowClient(t *testing.T) {
	// Given a registered client whose send buffer is already full
	hub := NewHub()
	go hub.Run()

	slow := &Client{hub: hub, send: make(chan []byte, 1)}
	slow.send <- []byte("already queued")

	hub.register <- slow
	time.Sleep(10 * time.Millisecond)
	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount() = %d before broadcast, want 1", got)
	}

	// When a broadcast finds no room for it
	if err := hub.Broadcast(Message{Type: "scan-progress"}); err != nil {
		t.Fatalf("Broadcast() failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// Then it stays connected and can catch up once it drains
	if got := hub.ClientCount(); got != 1 {
		t.Errorf("ClientCount() = %d after broadcast to a full buffer, want 1 (client was evicted)", got)
	}
}

// TestSendToClient_ReportsUndeliveredMessage covers silently-lost handoffs.
//
// Targeted sends dropped the message on a full buffer and returned nil, so a
// playback transfer could report success while the target tab never received
// the handoff: the initiating tab stops playing, the target never starts, and
// nothing short of a refresh recovers.
func TestSendToClient_ReportsUndeliveredMessage(t *testing.T) {
	// Given a registered client with no room in its send buffer
	hub := NewHub()
	go hub.Run()

	target := &Client{hub: hub, ID: "tab-1", send: make(chan []byte, 1)}
	target.send <- []byte("already queued")

	hub.register <- target
	time.Sleep(10 * time.Millisecond)

	// When a targeted message cannot be queued
	err := hub.SendToClient("tab-1", Message{Type: "playback-session"})

	// Then the caller is told, rather than being handed a silent success
	if !errors.Is(err, ErrClientBufferFull) {
		t.Errorf("SendToClient() error = %v, want ErrClientBufferFull", err)
	}
}

// TestSendToClient_UnknownClientIsNotAnError keeps the existing contract: a
// disconnected target is a no-op, not a failure.
func TestSendToClient_UnknownClientIsNotAnError(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	if err := hub.SendToClient("nobody-here", Message{Type: "ping"}); err != nil {
		t.Errorf("SendToClient() to an unknown client returned %v, want nil", err)
	}
}
