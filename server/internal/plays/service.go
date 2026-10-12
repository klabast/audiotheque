package plays

import (
	"errors"
	"fmt"
	"time"
)

// Period is how far back stats look.
type Period string

const (
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
	PeriodYear  Period = "year"
	PeriodAll   Period = "all"
)

// TopLimit is how many tracks, albums and artists the stats rank.
const TopLimit = 10

var ErrUnknownPeriod = errors.New("unknown period")

type Store interface {
	Stats(userID int64, since time.Time, limit int) (*Stats, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Stats(userID int64, period Period) (*Stats, error) {
	now := s.now()
	var since time.Time
	switch period {
	case PeriodWeek:
		since = now.AddDate(0, 0, -7)
	case PeriodMonth:
		since = now.AddDate(0, -1, 0)
	case PeriodYear:
		since = now.AddDate(-1, 0, 0)
	case PeriodAll:
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownPeriod, period)
	}
	return s.store.Stats(userID, since, TopLimit)
}
