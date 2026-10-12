package plays

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubStats struct {
	period Period
	stats  *Stats
	err    error
}

func (s *stubStats) Stats(userID int64, period Period) (*Stats, error) {
	s.period = period
	return s.stats, s.err
}

func newTestHandler(svc ServiceInterface) http.Handler {
	h := NewHandler(svc, func(r *http.Request) (int64, error) {
		if r.Header.Get("X-User") == "" {
			return 0, errors.New("unauthorized")
		}
		return 1, nil
	})
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func getStats(t *testing.T, h http.Handler, query string, user bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/stats"+query, nil)
	if user {
		req.Header.Set("X-User", "alice")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Stats(t *testing.T) {
	svc := &stubStats{stats: &Stats{
		Listens:          4,
		ListeningSeconds: 900,
		TopTracks:        []TrackCount{{TrackID: 7, Title: "Song", ArtistName: "A", AlbumID: 3, AlbumTitle: "Record", Plays: 4}},
	}}

	rec := getStats(t, newTestHandler(svc), "?period=week", true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if svc.period != PeriodWeek {
		t.Errorf("period = %q, want week", svc.period)
	}
	var got StatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Listens != 4 || got.ListeningSeconds != 900 || len(got.TopTracks) != 1 || got.TopTracks[0].AlbumTitle != "Record" {
		t.Errorf("body = %+v", got)
	}
	if got.TopAlbums == nil || got.TopArtists == nil {
		t.Errorf("empty lists must be [], not null: %+v", got)
	}
}

func TestHandler_StatsErrors(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		user   bool
		err    error
		status int
	}{
		{"needs a user", "?period=week", false, nil, http.StatusUnauthorized},
		{"unknown period", "?period=decade", true, ErrUnknownPeriod, http.StatusBadRequest},
		{"store failure", "?period=week", true, errors.New("db gone"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubStats{stats: &Stats{}, err: tt.err}

			rec := getStats(t, newTestHandler(svc), tt.query, tt.user)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}
