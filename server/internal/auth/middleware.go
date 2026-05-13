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

// GetAuthenticatedUser extracts and validates the JWT token from the request cookie,
// then returns the authenticated user.
// Returns ErrUnauthorized if the token is missing or invalid.
func GetAuthenticatedUser(r *http.Request, service *Service) (*User, error) {
	// Get token from cookie
	cookie, err := r.Cookie("audiod_token")
	if err != nil {
		return nil, ErrUnauthorized
	}

	// Validate token
	claims, err := ValidateToken(cookie.Value)
	if err != nil {
		log.Printf("Invalid token: %v", err)
		return nil, ErrUnauthorized
	}

	// Get user from database
	user, err := service.GetUserByID(claims.UserID)
	if err != nil {
		log.Printf("Failed to get user %d: %v", claims.UserID, err)
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
