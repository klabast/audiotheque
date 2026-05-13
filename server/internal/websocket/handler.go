package websocket

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

// checkOrigin enforces the WebSocket origin policy. Auth is via an httpOnly
// cookie, so an open origin check would expose us to cross-site WebSocket
// hijacking. Empty Origin is treated as a non-browser client (curl, MPD
// tools); browsers always send Origin on the upgrade.
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, allowed := range strings.Split(os.Getenv("AUDIOD_ALLOWED_ORIGINS"), ",") {
		allowed = strings.TrimSpace(allowed)
		if allowed != "" && strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

// UserIDGetter extracts user ID from request (injected from auth)
type UserIDGetter func(r *http.Request) (int64, error)

// Handler handles WebSocket HTTP requests
type Handler struct {
	hub       *Hub
	getUserID UserIDGetter
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub) *Handler {
	return &Handler{
		hub: hub,
	}
}

// SetUserIDGetter sets the function used to extract user ID from requests
func (h *Handler) SetUserIDGetter(getter UserIDGetter) {
	h.getUserID = getter
}

// RegisterRoutes registers all websocket routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ws", h.HandleWebSocket)
}

// HandleWebSocket upgrades HTTP connection to WebSocket
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Capture User-Agent before upgrade — once we hijack the connection the
	// http.Request's headers are still valid but it's cleaner to grab it now.
	userAgent := r.Header.Get("User-Agent")

	// Authenticate before upgrading. A userID=0 connection would still receive
	// every Hub.Broadcast and could submit incoming messages, so fail closed.
	if h.getUserID == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := h.getUserID(r)
	if err != nil || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Serve the WebSocket connection through the hub
	ServeWs(h.hub, conn, userID, userAgent)
}
