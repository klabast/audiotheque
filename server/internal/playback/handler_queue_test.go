package playback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func serveQueue(t *testing.T, svc *stubService, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	handler := NewHandler(svc, func(*http.Request) (int64, error) { return 1, nil })
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestHandleQueue_RoutesToTheService(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		check  func(t *testing.T, s *stubService)
	}{
		{"add tracks up next", "POST", "/api/playback/queue", `{"trackIds":[5,6],"position":"next"}`,
			func(t *testing.T, s *stubService) {
				if s.queueCall != "addTracks" || !slices.Equal(s.queueTrackIDs, []int64{5, 6}) || s.queuePosition != QueueNext {
					t.Errorf("got %s %v %s", s.queueCall, s.queueTrackIDs, s.queuePosition)
				}
			}},
		{"add an album at the end", "POST", "/api/playback/queue", `{"albumId":9,"position":"end"}`,
			func(t *testing.T, s *stubService) {
				if s.queueCall != "addAlbum" || s.queueAlbumID != 9 || s.queuePosition != QueueEnd {
					t.Errorf("got %s %d %s", s.queueCall, s.queueAlbumID, s.queuePosition)
				}
			}},
		{"remove an entry", "DELETE", "/api/playback/queue/abc", ``,
			func(t *testing.T, s *stubService) {
				if s.queueCall != "remove" || s.queueItemID != "abc" {
					t.Errorf("got %s %q", s.queueCall, s.queueItemID)
				}
			}},
		{"clear", "DELETE", "/api/playback/queue", ``,
			func(t *testing.T, s *stubService) {
				if s.queueCall != "clear" {
					t.Errorf("got %s", s.queueCall)
				}
			}},
		{"move an entry", "POST", "/api/playback/queue/abc/move", `{"index":2}`,
			func(t *testing.T, s *stubService) {
				if s.queueCall != "move" || s.queueItemID != "abc" || s.queueIndex != 2 {
					t.Errorf("got %s %q %d", s.queueCall, s.queueItemID, s.queueIndex)
				}
			}},
		{"play an entry", "POST", "/api/playback/queue/abc/play", ``,
			func(t *testing.T, s *stubService) {
				if s.queueCall != "play" || s.queueItemID != "abc" {
					t.Errorf("got %s %q", s.queueCall, s.queueItemID)
				}
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubService{}

			w := serveQueue(t, svc, tc.method, tc.path, tc.body)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
			}
			tc.check(t, svc)
		})
	}
}

func TestHandleQueue_RespondsWithQueueItems(t *testing.T) {
	w := serveQueue(t, &stubService{}, "DELETE", "/api/playback/queue", ``)

	var resp struct {
		Queue []QueueItemResponse `json:"queue"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []QueueItemResponse{{ID: "q1", TrackID: 7, AddedFrom: QueueOriginResponse{Type: "album", ID: 3}}}
	if !slices.Equal(resp.Queue, want) {
		t.Errorf("queue = %+v, want %+v", resp.Queue, want)
	}
}

func TestHandleAddToQueue_RejectsBadRequests(t *testing.T) {
	for _, body := range []string{
		`{"position":"end"}`,
		`{"trackIds":[],"position":"end"}`,
		`{"trackIds":[1],"albumId":2,"position":"end"}`,
		`{"trackIds":[1],"position":"later"}`,
		`{"trackIds":[1]}`,
		`not json`,
	} {
		t.Run(body, func(t *testing.T) {
			svc := &stubService{}

			w := serveQueue(t, svc, "POST", "/api/playback/queue", body)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if svc.queueCall != "" {
				t.Errorf("service was called: %s", svc.queueCall)
			}
		})
	}
}

func TestHandleQueue_MapsErrors(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{ErrQueueItemNotFound, http.StatusNotFound},
		{fmt.Errorf("get tracks to queue: %w", ErrTrackNotFound), http.StatusNotFound},
		{ErrNoSession, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			w := serveQueue(t, &stubService{err: tc.err}, "POST", "/api/playback/queue/abc/play", ``)

			if w.Code != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}
