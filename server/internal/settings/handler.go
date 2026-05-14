package settings

import (
	"encoding/json"
	"net/http"

	"audiod/internal/auth"
)

// ServiceInterface defines the methods the handler depends on.
type ServiceInterface interface {
	CreateDevice(name, deviceType, address string) (*Device, error)
	ListDevices() ([]Device, error)
	UpdateDevice(id, name, address string) (*Device, error)
	DeleteDevice(id string) error
	GetStreamingHostname() (string, error)
	SetStreamingHostname(hostname string) error
	IsAuthEnabled() (bool, error)
	SetAuthEnabled(enabled bool) error
}

type Handler struct {
	service     ServiceInterface
	authService *auth.Service
}

func NewHandler(service ServiceInterface, authService *auth.Service) *Handler {
	return &Handler{
		service:     service,
		authService: authService,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings/devices", h.HandleListDevices)
	mux.HandleFunc("POST /api/settings/devices", h.HandleCreateDevice)
	mux.HandleFunc("PUT /api/settings/devices/{id}", h.HandleUpdateDevice)
	mux.HandleFunc("DELETE /api/settings/devices/{id}", h.HandleDeleteDevice)
	mux.HandleFunc("GET /api/settings/streaming", h.HandleGetStreaming)
	mux.HandleFunc("PUT /api/settings/streaming", h.HandleSetStreaming)
	mux.HandleFunc("GET /api/settings/auth", h.HandleGetAuth)
	mux.HandleFunc("PUT /api/settings/auth", h.HandleSetAuth)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (*auth.User, error) {
	user, err := auth.AuthenticateRequest(w, r, h.authService)
	if err != nil {
		return nil, err
	}
	if err := auth.RequireAdmin(user); err != nil {
		return nil, err
	}
	return user, nil
}

// HandleListDevices handles GET /api/settings/devices
func (h *Handler) HandleListDevices(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	devices, err := h.service.ListDevices()
	if err != nil {
		http.Error(w, "failed to list devices", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(devices); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

type createDeviceRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

// HandleCreateDevice handles POST /api/settings/devices
func (h *Handler) HandleCreateDevice(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	var req createDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Address == "" {
		http.Error(w, "name and address are required", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = "mpd"
	}

	device, err := h.service.CreateDevice(req.Name, req.Type, req.Address)
	if err != nil {
		http.Error(w, "failed to create device", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(device); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

type updateDeviceRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// HandleUpdateDevice handles PUT /api/settings/devices/{id}
func (h *Handler) HandleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "device id is required", http.StatusBadRequest)
		return
	}

	var req updateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	device, err := h.service.UpdateDevice(id, req.Name, req.Address)
	if err != nil {
		if err == ErrDeviceNotFound {
			http.Error(w, "device not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to update device", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(device); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// HandleDeleteDevice handles DELETE /api/settings/devices/{id}
func (h *Handler) HandleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "device id is required", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteDevice(id); err != nil {
		http.Error(w, "failed to delete device", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type streamingResponse struct {
	Hostname string `json:"hostname"`
}

type setStreamingRequest struct {
	Hostname string `json:"hostname"`
}

// HandleGetStreaming handles GET /api/settings/streaming
func (h *Handler) HandleGetStreaming(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	hostname, err := h.service.GetStreamingHostname()
	if err != nil {
		http.Error(w, "failed to get streaming settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(streamingResponse{Hostname: hostname}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

type authSettingResponse struct {
	Enabled bool `json:"enabled"`
}

type setAuthRequest struct {
	Enabled bool `json:"enabled"`
}

// HandleGetAuth handles GET /api/settings/auth
// @Summary Get authentication-required setting
// @Description Reports whether browser login is currently required. Admin-only.
// @Tags settings
// @Produce json
// @Success 200 {object} authSettingResponse
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Router /settings/auth [get]
func (h *Handler) HandleGetAuth(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}
	enabled, err := h.service.IsAuthEnabled()
	if err != nil {
		http.Error(w, "failed to read auth setting", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(authSettingResponse{Enabled: enabled}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// HandleSetAuth handles PUT /api/settings/auth
// @Summary Set authentication-required setting
// @Description Toggle whether browser login is required. Admin-only. The
// @Description frontend gates this behind a sudo (re-confirm-password) modal
// @Description when disabling — sudo is enforced at the modal layer via
// @Description POST /auth/verify-password, not here.
// @Tags settings
// @Accept json
// @Produce json
// @Param request body setAuthRequest true "auth setting"
// @Success 200 {object} authSettingResponse
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Router /settings/auth [put]
func (h *Handler) HandleSetAuth(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}
	var req setAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.service.SetAuthEnabled(req.Enabled); err != nil {
		http.Error(w, "failed to save auth setting", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(authSettingResponse{Enabled: req.Enabled}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// HandleSetStreaming handles PUT /api/settings/streaming
func (h *Handler) HandleSetStreaming(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireAdmin(w, r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	var req setStreamingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	if err := h.service.SetStreamingHostname(req.Hostname); err != nil {
		http.Error(w, "failed to save streaming settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "streaming settings saved"}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
