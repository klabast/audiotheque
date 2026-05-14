package auth

import (
	"errors"
	"log"
	"net/http"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden: admin access required")
)

// GetAuthenticatedUser resolves the audiod_token cookie to a user via the
// DB-backed session table and returns the user.
// Returns ErrUnauthorized if the cookie is missing, the session is unknown,
// expired, or refers to a deleted user.
func GetAuthenticatedUser(r *http.Request, service *Service) (*User, error) {
	cookie, err := r.Cookie("audiod_token")
	if err != nil {
		return nil, ErrUnauthorized
	}

	user, err := service.ValidateSession(cookie.Value)
	if err != nil {
		// Unknown/expired sessions are the common case; only log the noisy
		// ones (e.g. DB errors) and return a clean Unauthorized.
		if !errors.Is(err, ErrSessionNotFound) {
			log.Printf("Session validation error: %v", err)
		}
		return nil, ErrUnauthorized
	}

	return user, nil
}

// RequireAdmin checks if the authenticated user is an admin.
// Returns ErrForbidden if the user is not an admin.
func RequireAdmin(user *User) error {
	if !user.IsAdmin {
		return ErrForbidden
	}
	return nil
}
