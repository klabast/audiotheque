package system

import (
	"encoding/json"
	"net/http"
)

type AuthService interface {
	DoesAdminUserExist() (bool, error)
}

type Handler struct {
	authService AuthService
}

func NewHandler(authService AuthService) *Handler {
	return &Handler{
		authService: authService,
	}
}

// RegisterRoutes registers all system routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/system", h.HandleSystem)
}

type SystemResponse struct {
	RequiresAdminUser bool   `json:"requiresAdminUser" example:"false"`
	Version           string `json:"version" example:"2.0"`
}

// @Summary System status
// @Description Returns system status including whether admin user setup is required
// @Tags system
// @Produce json
// @Success 200 {object} SystemResponse
// @Router /system [get]
// @ID getSystemStatus
func (h *Handler) HandleSystem(w http.ResponseWriter, r *http.Request) {
	adminExists, err := h.authService.DoesAdminUserExist()
	if err != nil {
		http.Error(w, "Failed to check system status", http.StatusInternalServerError)
		return
	}

	response := SystemResponse{
		RequiresAdminUser: !adminExists,
		Version:           "2.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
