package playback

import (
	"fmt"
	"slices"

	"github.com/google/uuid"
)

// AddTracksToQueue queues tracks up next or at the end, keeping their order.
// Every track must exist and be accessible, or nothing is queued.
func (s *Service) AddTracksToQueue(userID int64, trackIDs []int64, position QueuePosition) (*Session, error) {
	return s.editQueue(userID, func(session *Session) error {
		tracks, err := s.tracks.GetTracks(userID, trackIDs)
		if err != nil {
			return fmt.Errorf("get tracks to queue: %w", err)
		}
		insertIntoQueue(session, tracks, position)
		return nil
	})
}

// AddAlbumToQueue queues every track of an album, in album order.
func (s *Service) AddAlbumToQueue(userID, albumID int64, position QueuePosition) (*Session, error) {
	return s.editQueue(userID, func(session *Session) error {
		tracks, err := s.tracks.GetAlbumTracks(userID, albumID)
		if err != nil {
			return fmt.Errorf("get album tracks to queue: %w", err)
		}
		insertIntoQueue(session, tracks, position)
		return nil
	})
}

func (s *Service) RemoveFromQueue(userID int64, itemID string) (*Session, error) {
	return s.editQueue(userID, func(session *Session) error {
		i, err := queueIndex(session, itemID)
		if err != nil {
			return err
		}
		session.Queue = slices.Delete(session.Queue, i, i+1)
		return nil
	})
}

// ClearQueue empties the explicit queue. The source and what is playing now
// are left alone.
func (s *Service) ClearQueue(userID int64) (*Session, error) {
	return s.editQueue(userID, func(session *Session) error {
		session.Queue = []QueueItem{}
		return nil
	})
}

// MoveQueueItem moves an entry to index `to`, clamped to the queue's bounds.
func (s *Service) MoveQueueItem(userID int64, itemID string, to int) (*Session, error) {
	return s.editQueue(userID, func(session *Session) error {
		from, err := queueIndex(session, itemID)
		if err != nil {
			return err
		}
		item := session.Queue[from]
		session.Queue = slices.Delete(session.Queue, from, from+1)
		to = max(0, min(to, len(session.Queue)))
		session.Queue = slices.Insert(session.Queue, to, item)
		return nil
	})
}

// PlayFromQueue jumps straight to a queued entry. Entries queued before it are
// dropped, the way skipping ahead through them would have.
func (s *Service) PlayFromQueue(userID int64, itemID string) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}
	i, err := queueIndex(session, itemID)
	if err != nil {
		return nil, err
	}

	if session.Current != nil {
		session.History = append(session.History, session.Current.TrackID)
	}
	session.Current = &CurrentTrack{TrackID: session.Queue[i].TrackID}
	session.Queue = session.Queue[i+1:]
	session.State = StatePlaying

	if err := s.playOnDevice(session); err != nil {
		return nil, s.persistFailedAdvance(session, "from queue", err)
	}
	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}
	return session, nil
}

// editQueue runs a queue change against the user's session and persists it.
// Queue edits never touch the device: what is playing doesn't change.
func (s *Service) editQueue(userID int64, edit func(*Session) error) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}
	if err := edit(session); err != nil {
		return nil, err
	}
	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}
	return session, nil
}

func insertIntoQueue(session *Session, tracks []Track, position QueuePosition) {
	items := make([]QueueItem, len(tracks))
	for i, t := range tracks {
		items[i] = QueueItem{
			ID:          uuid.NewString(),
			TrackID:     t.ID,
			AddedFrom:   SourceTypeAlbum,
			AddedFromID: t.AlbumID,
		}
	}
	if position == QueueNext {
		session.Queue = append(items, session.Queue...)
		return
	}
	session.Queue = append(session.Queue, items...)
}

func queueIndex(session *Session, itemID string) (int, error) {
	i := slices.IndexFunc(session.Queue, func(item QueueItem) bool { return item.ID == itemID })
	if i < 0 {
		return 0, ErrQueueItemNotFound
	}
	return i, nil
}
