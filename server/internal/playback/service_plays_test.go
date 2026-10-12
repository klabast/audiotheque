package playback

import (
	"errors"
	"slices"
	"testing"
)

type recordedPlay struct {
	userID, trackID int64
	playedSeconds   int
}

type fakePlayRecorder struct {
	plays []recordedPlay
	err   error
}

func (f *fakePlayRecorder) RecordPlay(userID, trackID int64, playedSeconds int) error {
	f.plays = append(f.plays, recordedPlay{userID, trackID, playedSeconds})
	return f.err
}

// playsFixture is queueFixture with track 1 played for 200 seconds, track 20
// queued, and a play recorder attached.
func playsFixture(t *testing.T) (*Service, *mockSessionRepository, *fakePlayRecorder) {
	t.Helper()
	svc, repo, _ := queueFixture(t)
	repo.session.Current.Position = 200
	repo.session.Queue = []QueueItem{{ID: "q20", TrackID: 20}}
	recorder := &fakePlayRecorder{}
	svc.SetPlayRecorder(recorder)
	return svc, repo, recorder
}

func TestService_RecordsThePlayOfTheTrackItLeaves(t *testing.T) {
	tests := []struct {
		name  string
		leave func(*Service) error
	}{
		{"next", func(s *Service) error { _, err := s.Next(1); return err }},
		{"previous", func(s *Service) error { _, err := s.Previous(1); return err }},
		{"play from queue", func(s *Service) error { _, err := s.PlayFromQueue(1, "q20"); return err }},
		{"play an album", func(s *Service) error { _, err := s.PlayAlbumOnDevice(1, 200, 0, testDeviceID); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given track 1 has played for 200 seconds
			svc, _, recorder := playsFixture(t)

			// When playback moves on
			if err := tt.leave(svc); err != nil {
				t.Fatalf("leave: %v", err)
			}

			// Then that play is recorded
			want := []recordedPlay{{userID: 1, trackID: 1, playedSeconds: 200}}
			if !slices.Equal(recorder.plays, want) {
				t.Errorf("plays = %v, want %v", recorder.plays, want)
			}
		})
	}
}

func TestService_RecordsTheLastTrackWhenTheSourceRunsOut(t *testing.T) {
	// Given the last track of the source is playing
	svc, repo, recorder := playsFixture(t)
	repo.session.Queue = []QueueItem{}
	repo.session.Source.Remaining = []int64{}

	// When it ends
	if _, err := svc.Next(1); err != nil {
		t.Fatalf("Next: %v", err)
	}

	// Then it is recorded although nothing follows
	if len(recorder.plays) != 1 || recorder.plays[0].trackID != 1 {
		t.Errorf("plays = %v, want track 1", recorder.plays)
	}
}

func TestService_PrivateSessionRecordsNothing(t *testing.T) {
	// Given a private session
	svc, repo, recorder := playsFixture(t)
	repo.session.IsPrivate = true

	// When the track is skipped
	if _, err := svc.Next(1); err != nil {
		t.Fatalf("Next: %v", err)
	}

	// Then no play is recorded
	if len(recorder.plays) != 0 {
		t.Errorf("plays = %v, want none", recorder.plays)
	}
}

func TestService_FailingToRecordAPlayDoesNotStopPlayback(t *testing.T) {
	// Given the play log is failing
	svc, _, recorder := playsFixture(t)
	recorder.err = errors.New("disk full")

	// When the track is skipped
	session, err := svc.Next(1)

	// Then playback still moves on
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if session.Current == nil || session.Current.TrackID != 20 {
		t.Errorf("current = %+v, want track 20", session.Current)
	}
}
