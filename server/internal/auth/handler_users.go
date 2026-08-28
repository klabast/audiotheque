package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

type UsersListResponse struct {
	Users []UserResponse `json:"users"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin"`
}

type ResetUserPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// HandleListUsers handles GET /api/users
// @Summary List all users
// @Description Lists every user in the system. Admin-only.
// @Tags users
// @Produce json
// @Success 200 {object} UsersListResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Router /users [get]
// @ID listUsers
func (h *Handler) HandleListUsers(w http.ResponseWriter, r *http.Request, _ *User) {
	users, err := h.service.ListUsers()
	if err != nil {
		log.Printf("ListUsers failed: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}
	out := make([]UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, u.ToResponse())
	}
	writeJSON(w, http.StatusOK, UsersListResponse{Users: out})
}

// HandleCreateUser handles POST /api/users
// @Summary Create a new user
// @Description Creates a new user account. Admin-only. Once the first admin
// @Description exists, every subsequent user is created through this endpoint
// @Description (the /auth/setup endpoint stops working after first-run).
// @Tags users
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "New user credentials"
// @Success 201 {object} UserResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 409 {string} string "Conflict"
// @Router /users [post]
// @ID createUser
func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request, _ *User) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	// Username/password rules live in Service.CreateUser so the CLI and any
	// other caller get them too.
	user, err := h.service.CreateUser(req.Username, req.Password, req.IsAdmin)
	if err != nil {
		log.Printf("CreateUser failed: %v", err)
		switch {
		case errors.Is(err, ErrUsernameTaken):
			http.Error(w, "Username is already taken", http.StatusConflict)
		case errors.Is(err, ErrFirstUserMustBeAdmin):
			http.Error(w, "The first user must be an admin", http.StatusBadRequest)
		case isValidationError(err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusCreated, user.ToResponse())
}

// HandleDeleteUser handles DELETE /api/users/{id}
// @Summary Delete a user
// @Description Removes a user row. Admin-only. Refuses to delete the
// @Description calling user or the system's last admin.
// @Tags users
// @Param id path int true "User id"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/{id} [delete]
// @ID deleteUser
func (h *Handler) HandleDeleteUser(w http.ResponseWriter, r *http.Request, actor *User) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteUser(actor.ID, id); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			http.Error(w, "user not found", http.StatusNotFound)
		case errors.Is(err, ErrCannotDeleteSelf):
			http.Error(w, "Cannot delete the currently signed-in user", http.StatusBadRequest)
		case errors.Is(err, ErrCannotDeleteLastAdmin):
			http.Error(w, "Cannot delete the last admin user", http.StatusBadRequest)
		default:
			log.Printf("DeleteUser failed: %v", err)
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleResetUserPassword handles PUT /api/users/{id}/password
// @Summary Reset another user's password
// @Description Admin-only password reset that bypasses the current-password
// @Description check (which the self-service /auth/password endpoint enforces).
// @Description Every session of the target user is revoked.
// @Tags users
// @Accept json
// @Param id path int true "User id"
// @Param request body ResetUserPasswordRequest true "New password"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/{id}/password [put]
// @ID resetUserPassword
func (h *Handler) HandleResetUserPassword(w http.ResponseWriter, r *http.Request, _ *User) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}
	var req ResetUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.NewPassword == "" {
		http.Error(w, "new_password is required", http.StatusBadRequest)
		return
	}
	if err := h.service.AdminResetUserPassword(id, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			http.Error(w, "user not found", http.StatusNotFound)
		case isValidationError(err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			log.Printf("AdminResetUserPassword failed: %v", err)
			http.Error(w, "Failed to reset password", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}
