package server

import (
	"audiod/internal/websocket"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
)

func TestHealthEndpoint(t *testing.T) {
	hub := websocket.NewHub()
	srv := New(nil, nil, nil, nil, nil, hub) // nil handlers are fine for health endpoint
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got '%s'", w.Body.String())
	}
}

// TestSetUserIDGetter_ForwardsToWSHandler is the regression test for the
// multi-tab sync bug: once the server has a UserIDGetter wired, /api/ws
// upgrades register with the resolved userID, and per-user broadcasts reach
// the connection. Without the forwarder, BroadcastToUser would silently match
// no one (every client lands as userID=0).
func TestSetUserIDGetter_ForwardsToWSHandler(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	srv := New(nil, nil, nil, nil, nil, hub)
	srv.SetUserIDGetter(func(r *http.Request) (int64, error) {
		if r.Header.Get("X-User") == "alice" {
			return 42, nil
		}
		return 0, errors.New("unauthorized")
	})

	httpSrv := httptest.NewServer(srv.Router())
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/ws"
	header := http.Header{"X-User": []string{"alice"}}

	conn, _, err := gws.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) && hub.ClientCount() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client registered, got %d", hub.ClientCount())
	}

	// With the pre-fix bug (no getter wired), the registered client had
	// userID=0 and a BroadcastToUser(42, ...) would never reach it. Confirm
	// the message lands.
	if err := hub.BroadcastToUser(42, websocket.Message{Type: "test", Data: "hi"}); err != nil {
		t.Fatalf("broadcast: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v (broadcast did not reach the authenticated client)", err)
	}
	if !strings.Contains(string(payload), `"type":"test"`) {
		t.Fatalf("unexpected payload: %s", string(payload))
	}
}
