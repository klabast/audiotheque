package plays

import (
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	since time.Time
	limit int
}

func (f *fakeStore) Stats(userID int64, since time.Time, limit int) (*Stats, error) {
	f.since, f.limit = since, limit
	return &Stats{}, nil
}

func TestService_StatsCoverThePeriodUpToNow(t *testing.T) {
	now := time.Date(2026, 10, 12, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		period Period
		since  time.Time
	}{
		{PeriodWeek, now.AddDate(0, 0, -7)},
		{PeriodMonth, now.AddDate(0, -1, 0)},
		{PeriodYear, now.AddDate(-1, 0, 0)},
		{PeriodAll, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(string(tt.period), func(t *testing.T) {
			store := &fakeStore{}
			svc := NewService(store)
			svc.now = func() time.Time { return now }

			if _, err := svc.Stats(1, tt.period); err != nil {
				t.Fatalf("Stats: %v", err)
			}

			if !store.since.Equal(tt.since) || store.limit != TopLimit {
				t.Errorf("since = %v, limit = %d; want %v, %d", store.since, store.limit, tt.since, TopLimit)
			}
		})
	}
}

func TestService_RejectsUnknownPeriods(t *testing.T) {
	svc := NewService(&fakeStore{})

	_, err := svc.Stats(1, "decade")

	if !errors.Is(err, ErrUnknownPeriod) {
		t.Errorf("err = %v, want ErrUnknownPeriod", err)
	}
}
