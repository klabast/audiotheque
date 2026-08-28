package server

import (
	"audiod/internal/auth"
	"audiod/internal/library"
	"audiod/internal/playback"
	"audiod/internal/settings"
	"audiod/internal/system"
	"audiod/internal/websocket"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Server struct {
	authHandler      *auth.Handler
	systemHandler    *system.Handler
	libraryHandler   *library.Handler
	playbackHandler  *playback.Handler
	settingsHandler  *settings.Handler
	websocketHandler *websocket.Handler
	hub              *websocket.Hub
}

func New(authHandler *auth.Handler, systemHandler *system.Handler, libraryHandler *library.Handler, playbackHandler *playback.Handler, settingsHandler *settings.Handler, hub *websocket.Hub) *Server {
	return &Server{
		authHandler:      authHandler,
		systemHandler:    systemHandler,
		libraryHandler:   libraryHandler,
		playbackHandler:  playbackHandler,
		settingsHandler:  settingsHandler,
		websocketHandler: websocket.NewHandler(hub),
		hub:              hub,
	}
}

// SetUserIDGetter forwards an auth-aware user-ID extractor to the WebSocket
// handler so upgrades register the client under the authenticated user. Without
// this, every connection lands as userID=0 and per-user broadcasts (the whole
// point of BroadcastToUserExcept) reach no one — multi-tab sync silently dies.
func (s *Server) SetUserIDGetter(getter websocket.UserIDGetter) {
	s.websocketHandler.SetUserIDGetter(getter)
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check (kept in server as it's infrastructure)
	mux.HandleFunc("GET /health", s.handleHealth)

	// Register domain routes - each handler owns its routes. Handlers now wrap
	// their routes in auth middleware at registration time, which reads fields
	// off the handler, so a nil one has to be skipped rather than registered.
	if s.systemHandler != nil {
		s.systemHandler.RegisterRoutes(mux)
	}
	if s.authHandler != nil {
		s.authHandler.RegisterRoutes(mux)
	}
	if s.libraryHandler != nil {
		s.libraryHandler.RegisterRoutes(mux)
	}
	if s.playbackHandler != nil {
		s.playbackHandler.RegisterRoutes(mux)
	}
	if s.settingsHandler != nil {
		s.settingsHandler.RegisterRoutes(mux)
	}
	if s.websocketHandler != nil {
		s.websocketHandler.RegisterRoutes(mux)
	}

	// Static SPA fallback. Anything not matched by an API route is served
	// from AUDIOD_WEB_DIR (default /app/web in the Docker image; unset in
	// dev because the Vite dev server handles UI hosting). Routes that
	// don't map to a real file fall back to index.html so SvelteKit's
	// client router can take over.
	if webDir := webDirOrEmpty(); webDir != "" {
		mux.Handle("/", spaFileHandler(webDir))
	}

	return mux
}

func webDirOrEmpty() string {
	if v := os.Getenv("AUDIOD_WEB_DIR"); v != "" {
		return v
	}
	if _, err := os.Stat("/app/web/index.html"); err == nil {
		return "/app/web"
	}
	return ""
}

func spaFileHandler(webDir string) http.Handler {
	indexPath := filepath.Join(webDir, "index.html")
	fs := http.FileServer(http.Dir(webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Defence in depth: never serve API paths from the static handler.
		// (The mux pattern matching prevents this in practice — api routes
		// register longer prefixes — but if someone adds a route without
		// a leading slash this stops the mistake from leaking files.)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		full := filepath.Join(webDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
