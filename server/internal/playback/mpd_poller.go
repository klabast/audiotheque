// MPD position poller. Browser playback isn't polled — the browser tab
// pushes its own position over WebSocket via UpdatePositionFromClient.
// This file is MPD-specific by design (SongID, observedPlay state machine);
// don't generalize without first abstracting those MPD-specific concepts.

package playback

import (
	"context"
	"time"
)

// activeMPDSession holds the per-user state the poller maintains across
// ticks. The two correctness-critical fields are observedPlay and lastSongID:
//
//   - observedPlay starts false and flips true the first tick we see the
//     device in state=play for lastSongID. It gates the auto-advance branch
//     so a transient state=stop right after Play (URL still loading, or
//     the device refusing to decode) is never mistaken for track-end.
//   - lastSongID tracks MPD's per-Add identifier for the current song.
//     When MPD's reported songid changes, observedPlay resets — the next
//     song's first stop tick mustn't inherit the previous song's "we saw
//     it play" credit.
//
// expectedTrack is the session.Current.TrackID at the time trackMPDState
// last seeded this entry; it lets trackMPDState detect Service-side track
// transitions (Next, PlayAlbumOnDevice, Transfer to a different MPD) and
// reseed observedPlay/lastSongID without those methods having to know
// about poller internals.
type activeMPDSession struct {
	deviceID      string
	expectedTrack int64
	lastSongID    string
	observedPlay  bool
}

// trackMPDState updates the poller's set of users currently playing on an MPD
// device. Called from persistAndBroadcast so every state transition is reflected
// without polling-side bookkeeping. Preserves observedPlay/lastSongID across
// successive calls for the same (user, device, track) so transient persists
// don't reset the play-observed credit.
func (s *Service) trackMPDState(session *Session) {
	if session == nil {
		return
	}
	active := session.State == StatePlaying &&
		session.DeviceID != "" &&
		session.Current != nil
	if !active {
		s.activeMPDUsers.Delete(session.UserID)
		return
	}

	next := activeMPDSession{
		deviceID:      session.DeviceID,
		expectedTrack: session.Current.TrackID,
	}
	if existing, ok := s.activeMPDUsers.Load(session.UserID); ok {
		if prev, ok := existing.(activeMPDSession); ok &&
			prev.deviceID == next.deviceID &&
			prev.expectedTrack == next.expectedTrack {
			// Same user, device, and track as last time — preserve the
			// poller's accumulated observation. Without this, the persist
			// that fires every UpdatePosition would itself reset observedPlay
			// each tick, and the auto-advance gate would never close.
			next.lastSongID = prev.lastSongID
			next.observedPlay = prev.observedPlay
		}
	}
	s.activeMPDUsers.Store(session.UserID, next)
}

// StartMPDPolling runs a background goroutine that polls MPD devices for
// position changes and broadcasts updates. Stops when ctx is cancelled.
// Browser playback is not polled — the browser tab pushes its own position
// over WebSocket.
func (s *Service) StartMPDPolling(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.PollMPDPositions()
			}
		}
	}()
}

// PollMPDPositions polls every active MPD-bound session once. The state
// machine per active user:
//
//  1. Status() error → leave entry untouched (invariant: never lose
//     position because of a transient device error).
//  2. status.SongID changed (and non-empty) → MPD has loaded a different
//     song. Reset observedPlay; subsequent state assertions reference
//     the new song.
//  3. state=play → mirror elapsed into the session, latch observedPlay.
//  4. state=pause → no-op (clients own pause-state transitions).
//  5. state=stop:
//     - !observedPlay → device never started for this song. Skip.
//       This kills the audio-wz blast-loop where MPD lingers in stop
//       while loading the URL.
//     - observedPlay → real track-end. Pre-clear observedPlay to absorb
//       the inevitable extra stop tick during MPD's Clear+Add+Play, then
//       call Next(). The post-Next persistAndBroadcast hits trackMPDState
//       which seeds a fresh entry for the new track.
//  6. state="" (some MPD versions when fully idle) → conservative skip.
func (s *Service) PollMPDPositions() {
	s.activeMPDUsers.Range(func(key, value any) bool {
		userID, ok := key.(int64)
		if !ok {
			return true
		}
		entry, ok := value.(activeMPDSession)
		if !ok {
			return true
		}
		device, err := s.resolveDevice(entry.deviceID)
		if err != nil {
			s.logger.Debug("poll: resolve device failed", "userID", userID, "error", err)
			return true
		}
		status, err := device.Status()
		if err != nil {
			s.logger.Debug("poll: status failed", "userID", userID, "error", err)
			return true
		}

		// Track transitions on the device side (Add bumped songid). Reset
		// observedPlay so the new song must be observed playing in its own
		// right before its stop counts as track-end.
		if status.SongID != "" && status.SongID != entry.lastSongID {
			entry.lastSongID = status.SongID
			entry.observedPlay = false
		}

		switch status.State {
		case "play":
			entry.observedPlay = true
			s.activeMPDUsers.Store(userID, entry)
			s.UpdatePosition(userID, status.Elapsed)
		case "pause":
			s.activeMPDUsers.Store(userID, entry)
		case "stop":
			if !entry.observedPlay {
				// Device hasn't started this song yet. Don't advance, don't
				// touch position. Just persist the (possibly updated)
				// lastSongID so a later songid change is still detectable.
				s.activeMPDUsers.Store(userID, entry)
				s.logger.Debug("poll: stop without prior play; not advancing",
					"userID", userID, "deviceID", entry.deviceID)
				return true
			}
			// Real track-end. Pre-clear observedPlay so the inevitable
			// extra stop tick that MPD reports during Clear+Add+Play
			// doesn't trigger a second Next(). The post-Next trackMPDState
			// will reseed for the new track.
			entry.observedPlay = false
			s.activeMPDUsers.Store(userID, entry)
			if _, err := s.Next(userID); err != nil {
				s.logger.Debug("poll: auto-advance failed", "userID", userID, "error", err)
			}
		default:
			// Empty or unknown state: be conservative, do nothing.
			s.activeMPDUsers.Store(userID, entry)
		}
		return true
	})
}
