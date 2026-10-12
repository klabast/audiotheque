package playback

import (
	"errors"
	"slices"
	"testing"
)

// queueFixture is a session playing track 1 of album 100 (tracks 1–3), with
// album 200 (tracks 20–22) available to queue.
func queueFixture(t *testing.T) (*Service, *mockSessionRepository, *mockPlaybackDevice) {
	t.Helper()
	tracks := &mockTrackProvider{tracks: []Track{
		{ID: 1, AlbumID: 100}, {ID: 2, AlbumID: 100}, {ID: 3, AlbumID: 100},
		{ID: 20, AlbumID: 200}, {ID: 21, AlbumID: 200}, {ID: 22, AlbumID: 200},
	}}
	repo := &mockSessionRepository{session: &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current:  &CurrentTrack{TrackID: 1},
		Source:   Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{2, 3}},
		Queue:    []QueueItem{},
		History:  []int64{},
	}}
	device := &mockPlaybackDevice{state: "play"}
	svc := NewService(tracks, repo)
	svc.SetDeviceResolver(&mockDeviceResolver{devices: map[string]PlaybackDevice{testDeviceID: device}})
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://test/stream" })
	return svc, repo, device
}

func queuedTrackIDs(s *Session) []int64 {
	ids := make([]int64, len(s.Queue))
	for i, item := range s.Queue {
		ids[i] = item.TrackID
	}
	return ids
}

func queueItemFor(t *testing.T, s *Session, trackID int64) QueueItem {
	t.Helper()
	for _, item := range s.Queue {
		if item.TrackID == trackID {
			return item
		}
	}
	t.Fatalf("track %d not in queue %v", trackID, queuedTrackIDs(s))
	return QueueItem{}
}

func TestService_AddTracksToQueue(t *testing.T) {
	tests := []struct {
		name     string
		adds     [][]int64
		position QueuePosition
		want     []int64
	}{
		{"add to queue appends", [][]int64{{20}, {21}}, QueueEnd, []int64{20, 21}},
		{"play next stacks, latest first", [][]int64{{20}, {21}}, QueueNext, []int64{21, 20}},
		{"several tracks keep their order at the end", [][]int64{{20}, {21, 22}}, QueueEnd, []int64{20, 21, 22}},
		{"several tracks keep their order up next", [][]int64{{20}, {21, 22}}, QueueNext, []int64{21, 22, 20}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given an album playing
			svc, _, _ := queueFixture(t)

			// When tracks are queued
			var session *Session
			var err error
			for _, ids := range tc.adds {
				session, err = svc.AddTracksToQueue(1, ids, tc.position)
				if err != nil {
					t.Fatalf("AddTracksToQueue: %v", err)
				}
			}

			// Then the queue has them in order, and the source is untouched
			if got := queuedTrackIDs(session); !slices.Equal(got, tc.want) {
				t.Errorf("queue = %v, want %v", got, tc.want)
			}
			if session.Source.ID != 100 || !slices.Equal(session.Source.Remaining, []int64{2, 3}) {
				t.Errorf("source changed: %+v", session.Source)
			}
		})
	}
}

func TestService_AddTracksToQueue_RecordsTheTracksAlbum(t *testing.T) {
	svc, _, _ := queueFixture(t)

	session, err := svc.AddTracksToQueue(1, []int64{21}, QueueEnd)
	if err != nil {
		t.Fatalf("AddTracksToQueue: %v", err)
	}

	item := queueItemFor(t, session, 21)
	if item.AddedFrom != SourceTypeAlbum || item.AddedFromID != 200 {
		t.Errorf("origin = %s %d, want album 200", item.AddedFrom, item.AddedFromID)
	}
	if item.ID == "" {
		t.Error("queue item has no id")
	}
}

func TestService_AddTracksToQueue_UnknownTrack(t *testing.T) {
	svc, repo, _ := queueFixture(t)

	_, err := svc.AddTracksToQueue(1, []int64{20, 999}, QueueEnd)

	if !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("err = %v, want ErrTrackNotFound", err)
	}
	if len(repo.session.Queue) != 0 {
		t.Errorf("queue = %v, want nothing queued", queuedTrackIDs(repo.session))
	}
}

func TestService_AddTracksToQueue_NoSession(t *testing.T) {
	svc, repo, _ := queueFixture(t)
	repo.session = nil

	_, err := svc.AddTracksToQueue(1, []int64{20}, QueueEnd)

	if !errors.Is(err, ErrNoSession) {
		t.Fatalf("err = %v, want ErrNoSession", err)
	}
}

func TestService_AddAlbumToQueue(t *testing.T) {
	tests := []struct {
		name     string
		position QueuePosition
		want     []int64
	}{
		{"at the end", QueueEnd, []int64{2, 20, 21, 22}},
		{"up next", QueueNext, []int64{20, 21, 22, 2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given a track already queued
			svc, _, _ := queueFixture(t)
			if _, err := svc.AddTracksToQueue(1, []int64{2}, QueueEnd); err != nil {
				t.Fatalf("AddTracksToQueue: %v", err)
			}

			// When a whole album is queued
			session, err := svc.AddAlbumToQueue(1, 200, tc.position)
			if err != nil {
				t.Fatalf("AddAlbumToQueue: %v", err)
			}

			// Then its tracks are queued in album order
			if got := queuedTrackIDs(session); !slices.Equal(got, tc.want) {
				t.Errorf("queue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestService_RemoveFromQueue(t *testing.T) {
	// Given the same track queued twice
	svc, _, _ := queueFixture(t)
	session, err := svc.AddTracksToQueue(1, []int64{20, 21, 20}, QueueEnd)
	if err != nil {
		t.Fatalf("AddTracksToQueue: %v", err)
	}
	second := session.Queue[2]

	// When one of the two is removed
	session, err = svc.RemoveFromQueue(1, second.ID)
	if err != nil {
		t.Fatalf("RemoveFromQueue: %v", err)
	}

	// Then only that entry is gone
	if got := queuedTrackIDs(session); !slices.Equal(got, []int64{20, 21}) {
		t.Errorf("queue = %v, want [20 21]", got)
	}
}

func TestService_RemoveFromQueue_UnknownItem(t *testing.T) {
	svc, _, _ := queueFixture(t)

	_, err := svc.RemoveFromQueue(1, "nope")

	if !errors.Is(err, ErrQueueItemNotFound) {
		t.Fatalf("err = %v, want ErrQueueItemNotFound", err)
	}
}

func TestService_ClearQueue(t *testing.T) {
	// Given a queue while music plays
	svc, _, device := queueFixture(t)
	if _, err := svc.AddTracksToQueue(1, []int64{20, 21}, QueueEnd); err != nil {
		t.Fatalf("AddTracksToQueue: %v", err)
	}

	// When it is cleared
	session, err := svc.ClearQueue(1)
	if err != nil {
		t.Fatalf("ClearQueue: %v", err)
	}

	// Then the queue is empty, and the source and playback carry on
	if len(session.Queue) != 0 {
		t.Errorf("queue = %v, want empty", queuedTrackIDs(session))
	}
	if session.State != StatePlaying || session.Current.TrackID != 1 {
		t.Errorf("playback changed: state %s, current %d", session.State, session.Current.TrackID)
	}
	if !slices.Equal(session.Source.Remaining, []int64{2, 3}) {
		t.Errorf("source remaining = %v, want [2 3]", session.Source.Remaining)
	}
	if device.playCalled || device.stopCalled {
		t.Error("clearing the queue touched the device")
	}
}

func TestService_MoveQueueItem(t *testing.T) {
	tests := []struct {
		name string
		move int64
		to   int
		want []int64
	}{
		{"to the top", 22, 0, []int64{22, 20, 21}},
		{"to the bottom", 20, 2, []int64{21, 22, 20}},
		{"one down", 20, 1, []int64{21, 20, 22}},
		{"past the end lands last", 20, 10, []int64{21, 22, 20}},
		{"before the start lands first", 22, -3, []int64{22, 20, 21}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given three queued tracks
			svc, _, _ := queueFixture(t)
			session, err := svc.AddTracksToQueue(1, []int64{20, 21, 22}, QueueEnd)
			if err != nil {
				t.Fatalf("AddTracksToQueue: %v", err)
			}

			// When one moves
			session, err = svc.MoveQueueItem(1, queueItemFor(t, session, tc.move).ID, tc.to)
			if err != nil {
				t.Fatalf("MoveQueueItem: %v", err)
			}

			// Then the order follows
			if got := queuedTrackIDs(session); !slices.Equal(got, tc.want) {
				t.Errorf("queue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestService_PlayFromQueue(t *testing.T) {
	// Given three queued tracks
	svc, _, device := queueFixture(t)
	session, err := svc.AddTracksToQueue(1, []int64{20, 21, 22}, QueueEnd)
	if err != nil {
		t.Fatalf("AddTracksToQueue: %v", err)
	}

	// When the second one is played straight away
	session, err = svc.PlayFromQueue(1, queueItemFor(t, session, 21).ID)
	if err != nil {
		t.Fatalf("PlayFromQueue: %v", err)
	}

	// Then it plays, what was queued before it is dropped, and the rest waits
	if session.Current.TrackID != 21 {
		t.Errorf("current = %d, want 21", session.Current.TrackID)
	}
	if got := queuedTrackIDs(session); !slices.Equal(got, []int64{22}) {
		t.Errorf("queue = %v, want [22]", got)
	}
	if !slices.Equal(session.History, []int64{1}) {
		t.Errorf("history = %v, want [1]", session.History)
	}
	if !device.playCalled {
		t.Error("device was not told to play")
	}
	if !slices.Equal(session.Source.Remaining, []int64{2, 3}) {
		t.Errorf("source remaining = %v, want [2 3]", session.Source.Remaining)
	}
}

func TestService_PlayFromQueue_ResumesWhenPaused(t *testing.T) {
	svc, repo, device := queueFixture(t)
	repo.session.State = StatePaused
	session, err := svc.AddTracksToQueue(1, []int64{20}, QueueEnd)
	if err != nil {
		t.Fatalf("AddTracksToQueue: %v", err)
	}

	session, err = svc.PlayFromQueue(1, session.Queue[0].ID)
	if err != nil {
		t.Fatalf("PlayFromQueue: %v", err)
	}

	if session.State != StatePlaying || !device.playCalled {
		t.Errorf("state = %s, device played = %v; want playing", session.State, device.playCalled)
	}
}
