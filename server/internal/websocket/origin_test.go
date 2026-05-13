package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCheckOrigin pins down the WebSocket origin policy. The server uses
// httpOnly cookie auth, so an open Origin check would let any site mount a
// cross-site WebSocket hijack. The policy is:
//   - empty Origin → allowed (non-browser clients don't set it)
//   - Origin host matches request Host → allowed (same-origin)
//   - Origin in AUDIOD_ALLOWED_ORIGINS (comma-separated) → allowed
//   - everything else → rejected
func TestCheckOrigin(t *testing.T) {
	cases := []struct {
		name   string
		host   string
		origin string
		env    string
		want   bool
	}{
		{"same origin", "example.com:8080", "http://example.com:8080", "", true},
		{"same origin https", "music.example.com", "https://music.example.com", "", true},
		{"cross origin rejected by default", "example.com:8080", "http://evil.com", "", false},
		{"empty origin allowed for non-browser", "example.com:8080", "", "", true},
		{"configured origin allowed", "api.example.com", "http://localhost:5173", "http://localhost:5173", true},
		{"configured list allowed", "api.example.com", "http://localhost:5173", "http://other.test, http://localhost:5173", true},
		{"unconfigured origin rejected even with list", "api.example.com", "http://evil.com", "http://localhost:5173", false},
		{"malformed origin rejected", "example.com:8080", "not a url", "", false},
		{"origin host mismatch", "example.com:8080", "http://example.com:9000", "", false},
		{"empty AUDIOD_ALLOWED_ORIGINS entry ignored", "api.example.com", "http://evil.com", ",, ,", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AUDIOD_ALLOWED_ORIGINS", tc.env)
			r := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/api/ws", nil)
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if got := checkOrigin(r); got != tc.want {
				t.Errorf("checkOrigin() = %v, want %v (origin=%q host=%q env=%q)", got, tc.want, tc.origin, tc.host, tc.env)
			}
		})
	}
}
