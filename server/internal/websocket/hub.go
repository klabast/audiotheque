package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

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
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))
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

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

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

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			break
		}
	}
}

// ServeWs handles WebSocket requests from clients
func ServeWs(hub *Hub, conn *websocket.Conn, userID int64, userAgent string) {
	client := &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		ID:        fmt.Sprintf("c%d", hub.nextClientID.Add(1)),
		UserID:    userID,
		UserAgent: userAgent,
	}

	client.hub.register <- client

	// Start read and write pumps in separate goroutines
	go client.writePump()
	go client.readPump()
}
