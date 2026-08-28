package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A session cookie lives up to 90 days; it must carry Secure whenever the
// request arrived over TLS, and the operator must be able to force it for the
// reverse-proxy case.
func TestCookieSecure(t *testing.T) {
	tests := []struct {
		name          string
		tls           bool
		forwardedNote string
		trustProxy    bool
		override      string
		want          bool
	}{
		{name: "plain http", want: false},
		{name: "direct tls", tls: true, want: true},
		{name: "forwarded https is ignored untrusted", forwardedNote: "https", want: false},
		{name: "forwarded https behind trusted proxy", forwardedNote: "https", trustProxy: true, want: true},
		{name: "forwarded http behind trusted proxy", forwardedNote: "http", trustProxy: true, want: false},
		{name: "env forces on", override: "true", want: true},
		{name: "env forces off over tls", tls: true, override: "false", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(CookieSecureEnv, tc.override)
			if tc.trustProxy {
				t.Setenv(TrustedProxyEnv, "true")
			} else {
				t.Setenv(TrustedProxyEnv, "")
			}

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if tc.forwardedNote != "" {
				r.Header.Set("X-Forwarded-Proto", tc.forwardedNote)
			}

			if got := cookieSecure(r); got != tc.want {
				t.Errorf("cookieSecure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSetSessionCookie_UsesTheSessionWindow(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	for _, rememberMe := range []bool{false, true} {
		w := httptest.NewRecorder()
		setSessionCookie(w, r, "abc", rememberMe)

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected one cookie, got %d", len(cookies))
		}
		want := int(SessionWindowFor(rememberMe).Seconds())
		if cookies[0].MaxAge != want {
			t.Errorf("rememberMe=%v: MaxAge = %d, want %d", rememberMe, cookies[0].MaxAge, want)
		}
		if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Errorf("rememberMe=%v: expected HttpOnly + SameSite=Lax, got %+v", rememberMe, cookies[0])
		}
	}
}
