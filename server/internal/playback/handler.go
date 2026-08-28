package playback

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"audiod/internal/library"
)

// PlayRequest is the request body for playing content
type PlayRequest struct {
	AlbumID  int64  `json:"albumId,omitempty"`
	TrackID  int64  `json:"trackId,omitempty"`
	DeviceID string `json:"deviceId,omitempty"`
}

// TransferRequest is the request body for transferring playback
type TransferRequest struct {
	DeviceID string `json:"deviceId"`
}

// DeviceResponse is the JSON response for a device
type DeviceResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address,omitempty"`
	// IsCurrent is true for the browser tab that made the request. The UI
	// renders the localized "This Device" label for that row; every other row
	// shows its real (UA-derived or admin-set) Name. Server stays out of i18n.
	IsCurrent bool `json:"isCurrent,omitempty"`
}

// PauseRequest is the request body for pausing playback
type PauseRequest struct {
	Position int `json:"position"` // seconds into track
}

// SeekRequest is the request body for seeking
type SeekRequest struct {
	Position int `json:"position"` // seconds into track
}

// VolumeRequest is the request body for setting volume
type VolumeRequest struct {
	Volume int `json:"volume"` // 0-100
}

// CurrentTrackResponse is the JSON response for current track
type CurrentTrackResponse struct {
	TrackID  int64 `json:"trackId"`
	Position int   `json:"position"`
}

// SourceResponse is the JSON response for source info
type SourceResponse struct {
	Type      string  `json:"type"`
	ID        int64   `json:"id"`
	Remaining []int64 `json:"remaining"`
}

// SessionResponse is the JSON response for playback session
type SessionResponse struct {
	State              string                `json:"state"`
	Current            *CurrentTrackResponse `json:"current,omitempty"`
	Source             *SourceResponse       `json:"source,omitempty"`
	Queue              []int64               `json:"queue"`
	DeviceID           string                `json:"deviceId,omitempty"`
	DeviceVolumes      map[string]int        `json:"deviceVolumes,omitempty"`
	DeviceCapabilities *DeviceCapabilities   `json:"deviceCapabilities,omitempty"`
}

// DeviceCapabilities is a transient (non-persisted) hint about what the
// active device can do. Clients use it to adjust the UI — e.g. grey out a
// volume slider for an MPD configured with `mixer_type "none"` instead of
// letting the user move it and silently fail.
type DeviceCapabilities struct {
	// Volume reports whether the device accepts setvol. Defaults true; flips
	// false the first time the device returns ErrVolumeNotSupported.
	Volume bool `json:"volume"`
}

// CapabilitiesFn returns the capability hint for a given device ID, or nil
// to omit the hint. SessionToResponse callers pass this when they have
// access to the device resolver (handler, broadcaster wiring); pass nil
// from contexts that don't (test helpers, simple readers).
type CapabilitiesFn func(deviceID string) *DeviceCapabilities

// ServiceInterface defines what the handler needs from the service. Every
// operation works on an active session bound to a real device. Session
// creation (PlayAlbumOnDevice) requires the caller to supply a non-empty
// deviceID — see Handler.resolvePlayTarget for the "play here" inference.
type ServiceInterface interface {
	PlayAlbumOnDevice(userID, albumID, startTrackID int64, deviceID string) (*Session, error)
	GetSession(userID int64) (*Session, error)
	Pause(userID int64, position int) (*Session, error)
	Resume(userID int64) (*Session, error)
	Next(userID int64) (*Session, error)
	Previous(userID int64) (*Session, error)
	TransferPlayback(userID int64, targetDeviceID string) (*Session, error)
	SeekTrack(userID int64, position int) (*Session, error)
	SetVolume(userID int64, volume int) (*Session, error)
	// DeviceCapabilities reports what the named device can do. Device I/O
	// belongs to the service, which owns the resolver.
	DeviceCapabilities(deviceID string) *DeviceCapabilities
}

// TrackStore defines methods for retrieving tracks and checking access
type TrackStore interface {
	GetTrackByID(trackID int64) (*library.Track, error)
	UserHasLibraryAccess(userID, libraryID int64) (bool, error)
}

// UserIDGetter extracts user ID from request (injected from auth middleware)
type UserIDGetter func(r *http.Request) (int64, error)

// StreamTokenValidator validates a signed stream-scoped token. The trackID is
// passed from the URL path so the validator can ensure the token was minted
// for the same track (prevents reusing a token across tracks). Returns the
// userID encoded in the token on success.
type StreamTokenValidator func(token string, trackID int64) (int64, error)

// Handler handles playback HTTP requests
type Handler struct {
	service              ServiceInterface
	getUserID            UserIDGetter
	streamTokenValidator StreamTokenValidator
	trackStore           TrackStore
	deviceRegistry       DeviceRegistry
	browserRegistry      *BrowserDeviceRegistry
}

// NewHandler creates a new playback handler
func NewHandler(service ServiceInterface, getUserID UserIDGetter) *Handler {
	return &Handler{
		service:   service,
		getUserID: getUserID,
	}
}

// SetTrackStore sets the track store for streaming
func (h *Handler) SetTrackStore(store TrackStore) {
	h.trackStore = store
}

// SetStreamTokenValidator sets the validator used as a fallback when cookie
// auth fails on the stream endpoint. Used by MPD devices that fetch the
// stream URL without a session.
func (h *Handler) SetStreamTokenValidator(v StreamTokenValidator) {
	h.streamTokenValidator = v
}

// SetDeviceRegistry sets the device registry for device listing
func (h *Handler) SetDeviceRegistry(registry DeviceRegistry) {
	h.deviceRegistry = registry
}

// SetBrowserRegistry sets the ephemeral browser-tab registry. When set, the
// device-list endpoint includes connected browser tabs and "This Device" maps
// to the requesting tab's hub-assigned client ID instead of the empty
// placeholder.
func (h *Handler) SetBrowserRegistry(registry *BrowserDeviceRegistry) {
	h.browserRegistry = registry
}

// capabilitiesFor returns the CapabilitiesFn used to fill the session
// response's device hint. The service owns the device resolver and therefore
// the I/O; the handler only passes the function along.
func (h *Handler) capabilitiesFor() CapabilitiesFn {
	if h.service == nil {
		return nil
	}
	return h.service.DeviceCapabilities
}

// writeServiceError maps a service error to an accurate status code and a
// message safe to hand a client. The wrapped error is never echoed: it carries
// MPD host addresses and other internals.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoSession):
		http.Error(w, "No active session", http.StatusNotFound)
	case errors.Is(err, ErrNoDevice):
		http.Error(w, "No playback device assigned to session", http.StatusConflict)
	case errors.Is(err, ErrDeviceNotFound):
		http.Error(w, "Playback device not found", http.StatusNotFound)
	case errors.Is(err, ErrDeviceUnreachable):
		http.Error(w, "Playback device is unreachable", http.StatusBadGateway)
	case errors.Is(err, ErrAccessDenied):
		http.Error(w, "Access denied", http.StatusForbidden)
	default:
		http.Error(w, "Playback operation failed", http.StatusInternalServerError)
	}
}

// RegisterRoutes registers all playback routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/playback/play", h.HandlePlay)
	mux.HandleFunc("GET /api/playback/session", h.HandleGetSession)
	mux.HandleFunc("POST /api/playback/pause", h.HandlePause)
	mux.HandleFunc("POST /api/playback/resume", h.HandleResume)
	mux.HandleFunc("POST /api/playback/next", h.HandleNext)
	mux.HandleFunc("POST /api/playback/previous", h.HandlePrevious)
	mux.HandleFunc("POST /api/playback/transfer", h.HandleTransfer)
	mux.HandleFunc("POST /api/playback/seek", h.HandleSeek)
	mux.HandleFunc("POST /api/playback/volume", h.HandleVolume)
	mux.HandleFunc("GET /api/devices", h.HandleListDevices)
	mux.HandleFunc("GET /api/tracks/{id}/stream", h.HandleStreamTrack)
}

// HandlePlay handles POST /api/playback/play
// @Summary Start playback
// @Description Start playing an album
// @Tags playback
// @Accept json
// @Produce json
// @Param request body PlayRequest true "Play request"
// @Success 200 {object} SessionResponse
// @Failure 400 {string} string "Bad request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/play [post]
// @ID play
func (h *Handler) HandlePlay(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req PlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AlbumID == 0 {
		http.Error(w, "Must specify albumId", http.StatusBadRequest)
		return
	}

	// Invariant: every session names an addressable playback device. If the
	// caller didn't specify one, infer it from the WS hub client ID. If we
	// still can't, reject — the legacy "empty means this tab" sentinel is
	// gone, and silently guessing locality is what produced the dual-playback
	// bug across browser tabs.
	deviceID, err := h.resolvePlayTarget(r, userID, req.DeviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := h.service.PlayAlbumOnDevice(userID, req.AlbumID, req.TrackID, deviceID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// resolvePlayTarget enforces the unified-session invariant: any operation
// that creates or moves a session must name a real playback device. Returns
// the explicit body deviceID if set, otherwise falls back to the requesting
// tab's hub client ID (validated against the browser registry). If neither
// is available — e.g. an unauthenticated client, a tab whose WebSocket hasn't
// welcomed yet, or a stale forged ID — return an error so the handler can
// reject with 400.
func (h *Handler) resolvePlayTarget(r *http.Request, userID int64, explicitDeviceID string) (string, error) {
	if explicitDeviceID != "" {
		// MPD devices are shared by design and pass straight through. A
		// browser tab is not: it belongs to whoever opened it, so binding a
		// session to someone else's tab has to be refused.
		if !h.browserTabBelongsTo(explicitDeviceID, userID) {
			return "", errors.New("device is not addressable by this user")
		}
		return explicitDeviceID, nil
	}
	clientID := r.Header.Get("X-Audiod-Client-Id")
	if clientID == "" {
		return "", errors.New("no playback device assigned: pass a deviceId or open a WebSocket so the server can route to this tab")
	}
	if h.browserRegistry == nil {
		return "", errors.New("no playback device assigned: browser registry unavailable")
	}
	if _, ok := h.browserRegistry.Get(clientID); !ok {
		return "", errors.New("no playback device assigned: client id is not a registered browser tab")
	}
	if !h.browserTabBelongsTo(clientID, userID) {
		return "", errors.New("device is not addressable by this user")
	}
	return clientID, nil
}

// browserTabBelongsTo reports whether deviceID is safe for userID to target.
// True for anything that isn't a registered browser tab (MPD devices), and for
// tabs that are either the user's own or still anonymous — a tab whose
// WebSocket connected before the user authenticated has UserID 0.
func (h *Handler) browserTabBelongsTo(deviceID string, userID int64) bool {
	if h.browserRegistry == nil {
		return true
	}
	tab, ok := h.browserRegistry.Get(deviceID)
	if !ok {
		return true
	}
	return tab.UserID == 0 || tab.UserID == userID
}

// HandleGetSession handles GET /api/playback/session
// @Summary Get playback session
// @Description Get the current user's playback session
// @Tags playback
// @Produce json
// @Success 200 {object} SessionResponse
// @Success 204 "No active session"
// @Failure 401 {string} string "Unauthorized"
// @Router /playback/session [get]
// @ID getSession
func (h *Handler) HandleGetSession(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, err := h.service.GetSession(userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if session == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func SessionToResponse(s *Session, caps CapabilitiesFn) SessionResponse {
	resp := SessionResponse{
		State:         string(s.State),
		Queue:         make([]int64, len(s.Queue)),
		DeviceID:      s.DeviceID,
		DeviceVolumes: s.DeviceVolumes,
	}

	if s.Current != nil {
		resp.Current = &CurrentTrackResponse{
			TrackID:  s.Current.TrackID,
			Position: s.Current.Position,
		}
	}

	resp.Source = &SourceResponse{
		Type:      string(s.Source.Type),
		ID:        s.Source.ID,
		Remaining: s.Source.Remaining,
	}

	for i, q := range s.Queue {
		resp.Queue[i] = q.TrackID
	}

	if caps != nil && s.DeviceID != "" {
		resp.DeviceCapabilities = caps(s.DeviceID)
	}

	return resp
}

// HandleStreamTrack handles GET /api/tracks/{id}/stream
// @Summary Stream audio track
// @Description Stream an audio file with range request support
// @Tags tracks
// @Produce audio/mpeg,audio/flac,audio/wav,audio/mp4,audio/ogg
// @Param id path int true "Track ID"
// @Success 200 {file} binary "Audio stream"
// @Success 206 {file} binary "Partial content (range request)"
// @Failure 400 {string} string "Invalid track ID"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Track not found"
// @Router /tracks/{id}/stream [get]
// @ID streamTrack
func (h *Handler) HandleStreamTrack(w http.ResponseWriter, r *http.Request) {
	// 1. Parse track ID first — needed for token-scoped auth fallback.
	trackIDStr := r.PathValue("id")
	trackID, err := strconv.ParseInt(trackIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	// 2. Authenticate. Try cookie auth first (browser tabs); fall back to a
	// signed ?token= for MPD devices that have no session. The token is
	// scoped to a specific trackID so a leak only exposes one track.
	userID, err := h.getUserID(r)
	if err != nil {
		token := r.URL.Query().Get("token")
		if token == "" || h.streamTokenValidator == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err = h.streamTokenValidator(token, trackID)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// 3. Get track
	track, err := h.trackStore.GetTrackByID(trackID)
	if err != nil || track == nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	// 4. Check user has access to this track's library
	hasAccess, err := h.trackStore.UserHasLibraryAccess(userID, track.LibraryID)
	if err != nil || !hasAccess {
		// Return 404 to not leak existence of tracks user can't access
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	// 5. Open audio file
	file, err := os.Open(track.FilePath)
	if err != nil {
		http.Error(w, "Track file not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// 6. Get file info for ServeContent
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// 7. Set content type based on codec
	contentType := getAudioContentType(track.Codec)
	w.Header().Set("Content-Type", contentType)

	// 8. Use http.ServeContent for range request support (seeking)
	http.ServeContent(w, r, track.FileName, stat.ModTime(), file)
}

// HandlePause handles POST /api/playback/pause
// @Summary Pause playback
// @Description Pause playback and save current position
// @Tags playback
// @Accept json
// @Produce json
// @Param request body PauseRequest true "Pause request with position"
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or device"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/pause [post]
// @ID pause
func (h *Handler) HandlePause(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req PauseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := h.service.Pause(userID, req.Position)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleResume handles POST /api/playback/resume
// @Summary Resume playback
// @Description Resume playback from current position
// @Tags playback
// @Produce json
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or device"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/resume [post]
// @ID resume
func (h *Handler) HandleResume(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, err := h.service.Resume(userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleNext handles POST /api/playback/next
// @Summary Skip to next track
// @Description Advance to the next track in queue or source
// @Tags playback
// @Produce json
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or device"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/next [post]
// @ID next
func (h *Handler) HandleNext(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, err := h.service.Next(userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandlePrevious handles POST /api/playback/previous
// @Summary Go to previous track
// @Description Go back to previous track or restart current track
// @Tags playback
// @Produce json
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or device"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/previous [post]
// @ID previous
func (h *Handler) HandlePrevious(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, err := h.service.Previous(userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleTransfer handles POST /api/playback/transfer
// @Summary Transfer playback
// @Description Transfer playback to a different device
// @Tags playback
// @Accept json
// @Produce json
// @Param request body TransferRequest true "Transfer request"
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or device"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/transfer [post]
// @ID transferPlayback
func (h *Handler) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Same invariant as HandlePlay: a transfer must name a real device.
	// Empty body deviceId is interpreted as "to me" — derived from the WS
	// client ID. No anonymous "to the browser" handoffs.
	deviceID, err := h.resolvePlayTarget(r, userID, req.DeviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := h.service.TransferPlayback(userID, deviceID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleSeek handles POST /api/playback/seek
// @Summary Seek to position
// @Description Seek to a position in the current track
// @Tags playback
// @Accept json
// @Produce json
// @Param request body SeekRequest true "Seek request"
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or device"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/seek [post]
// @ID seek
func (h *Handler) HandleSeek(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req SeekRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := h.service.SeekTrack(userID, req.Position)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleVolume handles POST /api/playback/volume
// @Summary Set volume
// @Description Set the playback volume
// @Tags playback
// @Accept json
// @Produce json
// @Param request body VolumeRequest true "Volume request"
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or device"
// @Failure 409 {string} string "Session has no playback device"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/volume [post]
// @ID setVolume
func (h *Handler) HandleVolume(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req VolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := h.service.SetVolume(userID, req.Volume)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleListDevices handles GET /api/devices
// @Summary List devices
// @Description List available playback devices for the current user
// @Tags devices
// @Produce json
// @Success 200 {array} DeviceResponse
// @Failure 401 {string} string "Unauthorized"
// @Router /devices [get]
// @ID listDevices
func (h *Handler) HandleListDevices(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var mpdDevices []Device
	if h.deviceRegistry != nil {
		mpdDevices, err = h.deviceRegistry.ListByUser(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Browser tabs are addressable devices via the ephemeral registry. The
	// requesting tab tells us its own hub client ID via X-Audiod-Client-Id; we
	// flag that row with IsCurrent=true so the UI renders the localized
	// "This Device" label for it. Names are returned verbatim — no server-side
	// i18n, no synthetic stand-in row. Under the unified-session invariant
	// every device has a real ID; if the requester isn't registered the UI
	// surfaces that honestly (loading state) rather than fabricating a row.
	thisClientID := r.Header.Get("X-Audiod-Client-Id")

	resp := []DeviceResponse{}

	if h.browserRegistry != nil {
		browsers := h.browserRegistry.ListByUser(userID)
		// Put the current device first, then the rest in registration order.
		for _, b := range browsers {
			if b.ClientID == thisClientID {
				resp = append(resp, DeviceResponse{ID: b.ClientID, Name: b.Name, Type: "browser", IsCurrent: true})
				break
			}
		}
		for _, b := range browsers {
			if b.ClientID == thisClientID {
				continue
			}
			resp = append(resp, DeviceResponse{ID: b.ClientID, Name: b.Name, Type: "browser"})
		}
	}

	for _, d := range mpdDevices {
		resp = append(resp, DeviceResponse{
			ID:   d.ID,
			Name: d.Name,
			Type: string(d.Type),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// getAudioContentType returns the MIME type for an audio codec
func getAudioContentType(codec string) string {
	switch strings.ToLower(codec) {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "aac", "m4a":
		return "audio/mp4"
	case "ogg", "vorbis":
		return "audio/ogg"
	case "wav":
		return "audio/wav"
	case "aiff", "aif":
		return "audio/aiff"
	case "dsf":
		return "audio/dsf"
	default:
		return "application/octet-stream"
	}
}
