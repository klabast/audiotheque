package auth

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden: admin access required")
)

// TrustedProxyEnv marks the server as sitting behind a reverse proxy that
// sets X-Forwarded-For / X-Forwarded-Proto. Off by default: with no proxy in
// front those headers are attacker-controlled, and honouring them lets anyone
// holding a stolen cookie disguise a session's origin in the Active Devices
// list.
const TrustedProxyEnv = "AUDIOD_TRUSTED_PROXY"

func trustProxyHeaders() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(TrustedProxyEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// clientIP extracts the caller's IP as a bare host, without a port. The single
// implementation used both for the session row's last_ip and for renewal, so a
// session's recorded IP keeps one format for its whole life.
func clientIP(r *http.Request) string {
	if trustProxyHeaders() {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := forwarded
			if comma := strings.Index(forwarded, ","); comma >= 0 {
				first = forwarded[:comma]
			}
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// GetAuthenticatedUser resolves the audiod_token cookie to a user via the
// DB-backed session table and returns the user. Sliding renewal fires
// internally — see Service.ValidateSession — but no Set-Cookie is written,
// so the browser's cookie expiry can drift from the bumped DB row.
//
// HTTP handlers that own a ResponseWriter should call AuthenticateRequest
// instead, which re-issues the cookie so renewal is visible client-side.
// This shorter form stays for the WebSocket upgrade path (which only owns
// the request) and any caller that explicitly doesn't want cookie writes.
//
// When auth is disabled (Service.AuthEnabled() == false) the cookie is
// ignored entirely and the canonical admin is returned. That is a listening
// convenience only: anything that grants privilege must go through
// AuthenticateSession / RequireRealAdmin instead.
func GetAuthenticatedUser(r *http.Request, service *Service) (*User, error) {
	if !service.AuthEnabled() {
		return service.GetCanonicalAdmin()
	}
	user, _, err := getAuthenticatedUserAndSession(r, service)
	return user, err
}

// AuthenticateRequest is the same as GetAuthenticatedUser but also re-issues
// the audiod_token cookie on success, so the browser's Max-Age tracks the
// session's (possibly bumped) expires_at. HTTP handlers should prefer this
// over GetAuthenticatedUser.
//
// In auth-disabled mode there is no session to renew, so no Set-Cookie is
// written — the canonical admin is returned as-is.
func AuthenticateRequest(w http.ResponseWriter, r *http.Request, service *Service) (*User, error) {
	if !service.AuthEnabled() {
		return service.GetCanonicalAdmin()
	}
	return AuthenticateSession(w, r, service)
}

// AuthenticateSession resolves the audiod_token cookie to a genuinely
// signed-in user and refreshes the cookie. Unlike AuthenticateRequest it
// ignores the auth-disabled toggle: turning auth off means "don't ask for a
// password to listen to music", not "any caller on the network may
// reprovision this server".
func AuthenticateSession(w http.ResponseWriter, r *http.Request, service *Service) (*User, error) {
	user, sess, err := getAuthenticatedUserAndSession(r, service)
	if err != nil {
		return nil, err
	}
	setSessionCookie(w, r, sess.ID, sess.RememberMe)
	return user, nil
}

func getAuthenticatedUserAndSession(r *http.Request, service *Service) (*User, *Session, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, nil, ErrUnauthorized
	}

	user, sess, err := service.ValidateSession(cookie.Value, clientIP(r))
	if err != nil {
		// Unknown/expired sessions are the common case; only log the noisy
		// ones (e.g. DB errors) and return a clean Unauthorized.
		if !errors.Is(err, ErrSessionNotFound) {
			log.Printf("Session validation error: %v", err)
		}
		return nil, nil, ErrUnauthorized
	}

	return user, sess, nil
}

// RequireAdmin checks if the authenticated user is an admin.
// Returns ErrForbidden if the user is not an admin.
func RequireAdmin(user *User) error {
	if !user.IsAdmin {
		return ErrForbidden
	}
	return nil
}

// RequireRealAdmin resolves a genuinely signed-in admin from the request,
// regardless of the auth-disabled toggle. Every privilege-granting surface
// (user management, password change/reset, session revocation) goes through
// this rather than RequireAdmin-on-AuthenticateRequest.
func RequireRealAdmin(w http.ResponseWriter, r *http.Request, service *Service) (*User, error) {
	user, err := AuthenticateSession(w, r, service)
	if err != nil {
		return nil, err
	}
	if err := RequireAdmin(user); err != nil {
		return nil, err
	}
	return user, nil
}

// AuthedHandlerFunc is an HTTP handler that only ever runs with a resolved
// user, so it never has to deal with the unauthenticated case itself.
type AuthedHandlerFunc func(w http.ResponseWriter, r *http.Request, user *User)

// RequireUser adapts an AuthedHandlerFunc into an http.HandlerFunc, rejecting
// the request before next runs if no valid session is present.
//
// Route registration is the only place authentication is decided, so a route
// is protected by how it is registered rather than by what its handler
// remembers to do.
func RequireUser(service *Service, next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := AuthenticateRequest(w, r, service)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		next(w, r, user)
	}
}

// RequireAdminUser is RequireUser plus an admin check.
func RequireAdminUser(service *Service, next AuthedHandlerFunc) http.HandlerFunc {
	return RequireUser(service, func(w http.ResponseWriter, r *http.Request, user *User) {
		if err := RequireAdmin(user); err != nil {
			WriteAuthError(w, err)
			return
		}
		next(w, r, user)
	})
}

// RequireRealUser demands a real signed-in session even when auth is
// disabled. Use it for anything acting on credentials or sessions.
func RequireRealUser(service *Service, next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := AuthenticateSession(w, r, service)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		next(w, r, user)
	}
}

// RequireRealAdminUser is RequireRealUser plus an admin check — the
// route-registration form of RequireRealAdmin.
func RequireRealAdminUser(service *Service, next AuthedHandlerFunc) http.HandlerFunc {
	return RequireRealUser(service, func(w http.ResponseWriter, r *http.Request, user *User) {
		if err := RequireAdmin(user); err != nil {
			WriteAuthError(w, err)
			return
		}
		next(w, r, user)
	})
}

// WriteAuthError maps an authentication or authorization error onto a response
// with a generic message, so internal error text never reaches the client.
func WriteAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrForbidden) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
