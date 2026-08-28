package auth

import (
	"net/http"
	"os"
	"strings"
)

// SessionCookieName is the cookie carrying the opaque session id.
const SessionCookieName = "audiod_token"

// CookieSecureEnv forces the Secure attribute on or off. Needed for the
// reverse-proxy case where TLS terminates upstream and the proxy does not
// (or is not trusted to) set X-Forwarded-Proto.
const CookieSecureEnv = "AUDIOD_COOKIE_SECURE"

// cookieSecure decides the Secure attribute for the session cookie from the
// request's transport, with an explicit env override. Forwarded headers only
// count when a proxy is trusted — see TrustedProxyEnv.
func cookieSecure(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(CookieSecureEnv))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	if r.TLS != nil {
		return true
	}
	return trustProxyHeaders() && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// setSessionCookie writes the session cookie. Its Max-Age comes from
// SessionWindowFor, the same source CreateSession and sliding renewal use, so
// the browser's expiry can't drift from the session row's.
func setSessionCookie(w http.ResponseWriter, r *http.Request, value string, rememberMe bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(SessionWindowFor(rememberMe).Seconds()),
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// currentSessionID returns the raw session id from the request cookie, or ""
// if no cookie is present.
func currentSessionID(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func sessionContextFromRequest(r *http.Request, rememberMe bool) SessionContext {
	return SessionContext{
		RememberMe: rememberMe,
		UserAgent:  r.Header.Get("User-Agent"),
		IP:         clientIP(r),
	}
}
