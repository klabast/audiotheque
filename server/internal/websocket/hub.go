package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Keepalive timing knobs, stored as atomic nanosecond counts so tests can
// override them while background goroutines from previous tests are still
// reading. Production reads via the get* helpers below.
//
// pongWait is the read deadline that ratchets forward each time we see a
// pong. pingInterval is how often we send a ping; it must be comfortably
// less than pongWait so a missed pong has time to be detected before the
// deadline fires. writeWait is a per-write deadline that prevents a stuck
// peer from blocking the write pump forever.
//
// Defaults: 60s pong wait, 54s ping (10% headroom), 10s write — the
// canonical gorilla/websocket recipe. Anything proxied tends to die
// somewhere between 60–120s of TCP idle, so 54s gives one reliable ping
// per minute on the wire.
var (
	pingInterval atomic.Int64
	pongWait     atomic.Int64
	writeWait    atomic.Int64
)

func init() {
	pingInterval.Store(int64(54 * time.Second))
	pongWait.Store(int64(60 * time.Second))
	writeWait.Store(int64(10 * time.Second))
}

func getPingInterval() time.Duration { return time.Duration(pingInterval.Load()) }
func getPongWait() time.Duration     { return time.Duration(pongWait.Load()) }
func getWriteWait() time.Duration    { return time.Duration(writeWait.Load()) }

// Message represents a WebSocket message to be broadcast
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// Client represents a WebSocket client connection. ID is assigned at
// registration time and is unique for the lifetime of the hub; it lets
// callers exclude a sender from a broadcast.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	ID        string
	UserID    int64
	UserAgent string
}

// IncomingMessageHandler handles messages received from WebSocket clients.
// clientID identifies the connection for sender-exclusion when broadcasting.
type IncomingMessageHandler func(userID int64, clientID string, msg Message)

// LifecycleHandler fires when a client registers or unregisters with the hub.
// Used by the playback layer to maintain an ephemeral browser-device registry
// so other tabs can transfer playback to this connection.
type LifecycleHandler func(c *Client)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Handler for incoming client messages
	onMessage IncomingMessageHandler

	// Lifecycle hooks fired on register / unregister. Optional.
	onRegister   LifecycleHandler
	onUnregister LifecycleHandler

	// Monotonic counter used to assign unique client IDs.
	nextClientID atomic.Uint64
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop (should be called in a goroutine)
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			// A reconnect (new tab connection reusing the same client-supplied
			// ID) can race ahead of the old connection's teardown: the new
			// register message may be processed before the old connection's
			// dead socket is even noticed. Without eviction here, both
			// connections would sit in h.clients under the same ID — so
			// SendToClient(id) (used for the client-id welcome) could pick the
			// stale, dying connection instead of the live one, and the new tab
			// would never learn its own ID. Evict any existing client with the
			// same non-empty ID before admitting the new one.
			h.mu.Lock()
			var stale *Client
			if client.ID != "" {
				for existing := range h.clients {
					if existing.ID == client.ID {
						stale = existing
						break
					}
				}
			}
			if stale != nil {
				delete(h.clients, stale)
				close(stale.send)
			}
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))
			if stale != nil {
				stale.conn.Close()
				if h.onUnregister != nil {
					h.onUnregister(stale)
				}
			}
			if h.onRegister != nil {
				h.onRegister(client)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			deleted := false
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				deleted = true
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))
			if deleted && h.onUnregister != nil {
				h.onUnregister(client)
			}

		case message := <-h.broadcast:
			// Collect stuck clients under RLock, then upgrade to Lock to
			// delete and close. Mutating the map under RLock raced with
			// concurrent BroadcastToUser readers; closing client.send while
			// another reader was mid-send caused panics.
			var stuck []*Client
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					stuck = append(stuck, client)
				}
			}
			h.mu.RUnlock()
			if len(stuck) > 0 {
				h.mu.Lock()
				for _, client := range stuck {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.broadcast <- data
	return nil
}

// BroadcastToUser sends a message to every client of the given user.
func (h *Hub) BroadcastToUser(userID int64, msg Message) error {
	return h.BroadcastToUserExcept(userID, "", msg)
}

// BroadcastToUserExcept sends a message to every client of the given user
// except the one whose ID matches exceptClientID. Pass "" to broadcast to all.
// This avoids echoing a value back to the client that just sent it (e.g. the
// browser's own playback-position tick).
func (h *Hub) BroadcastToUserExcept(userID int64, exceptClientID string, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.UserID != userID {
			continue
		}
		if exceptClientID != "" && client.ID == exceptClientID {
			continue
		}
		select {
		case client.send <- data:
		default:
			// Client's send buffer is full
		}
	}
	return nil
}

// SetMessageHandler sets the handler for incoming client messages
func (h *Hub) SetMessageHandler(handler IncomingMessageHandler) {
	h.onMessage = handler
}

// SetLifecycleHandlers sets the on-register / on-unregister callbacks. Either
// may be nil. The callbacks fire from the hub's goroutine, so they must not
// block — push work to a goroutine if it might.
func (h *Hub) SetLifecycleHandlers(onReg, onUnreg LifecycleHandler) {
	h.onRegister = onReg
	h.onUnregister = onUnreg
}

// SendToClient sends a message to a single client identified by clientID. It
// is a no-op (returns nil) if no matching client is connected. Used to push
// targeted messages such as the per-connection client-id welcome and
// transfer-target handoffs.
func (h *Hub) SendToClient(clientID string, msg Message) error {
	if clientID == "" {
		return nil
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.ID != clientID {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
		return nil
	}
	return nil
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// readPump pumps messages from the WebSocket connection to the hub.
//
// The read deadline is the keepalive enforcement point: every pong
// received from the peer ratchets it forward by pongWait. If pings stop
// being answered (dead intermediary, sleeping laptop, broken NAT) the
// deadline expires, ReadMessage returns an error, and we unregister.
// Without this, a TCP connection severed silently upstream would keep
// the hub believing a client is alive — and orphan its persisted
// session ID, surfacing in the UI as a phantom "Remote device".
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	if err := c.conn.SetReadDeadline(time.Now().Add(getPongWait())); err != nil {
		log.Printf("WebSocket set read deadline: %v", err)
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(getPongWait()))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// Parse and route incoming messages
		var msg Message
		if err := json.Unmarshal(message, &msg); err == nil && c.hub.onMessage != nil {
			c.hub.onMessage(c.UserID, c.ID, msg)
		} else if err != nil {
			log.Printf("Failed to parse WebSocket message: %v", err)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
//
// The ping ticker is the other half of the keepalive: it produces frames
// on the wire every pingInterval whether or not there is application
// traffic, which (a) gives the peer something to pong so the read
// deadline keeps ratcheting and (b) keeps the connection visible to
// every NAT/proxy/idle-killer between us and the browser.
//
// The hub closes c.send to signal unregister; we treat that as "send a
// close frame and return" so the read pump exits cleanly too.
func (c *Client) writePump() {
	ticker := time.NewTicker(getPingInterval())
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(getWriteWait())); err != nil {
				return
			}
			if !ok {
				// Hub closed the channel. Best-effort close frame, then exit.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(getWriteWait())); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// clientUUIDPattern matches a v4-shaped UUID (loosely — we don't check the
// version/variant nibbles, just the hyphenated hex shape). It's just an
// admission filter for a client-supplied identifier, not a security
// boundary, so leniency here is fine.
var clientUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// browserClientIDPrefix marks IDs assigned from a client-supplied UUID (a
// stable per-tab identity persisted in sessionStorage) so they can never
// collide with the hub's own "c<n>" counter format or with MPD device IDs.
const browserClientIDPrefix = "b-"

// ServeWs handles WebSocket requests from clients. requestedID is the
// client-supplied stable identifier (from sessionStorage, sent as a query
// param on the WS URL) that lets a browser tab survive a network change
// (e.g. LAN→WLAN) without losing its playback session — the old hub-assigned
// "c<n>" ID was a fresh, unpredictable value on every reconnect. If
// requestedID isn't a valid UUID (absent, malformed, or from an older
// client build), we fall back to the old monotonic counter.
func ServeWs(hub *Hub, conn *websocket.Conn, userID int64, userAgent string, requestedID string) {
	id := fmt.Sprintf("c%d", hub.nextClientID.Add(1))
	if clientUUIDPattern.MatchString(requestedID) {
		id = browserClientIDPrefix + requestedID
	}
	client := &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		ID:        id,
		UserID:    userID,
		UserAgent: userAgent,
	}

	client.hub.register <- client

	// Start read and write pumps in separate goroutines
	go client.writePump()
	go client.readPump()
}
