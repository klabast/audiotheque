// MPD position poller. Browser playback isn't polled — the browser tab
// pushes its own position over WebSocket via UpdatePositionFromClient.
// This file is MPD-specific by design (SongID, observedPlay state machine);
// don't generalize without first abstracting those MPD-specific concepts.

package playback

import (
	"context"
	"sync"
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

// mpdTracker holds the poller's per-user bookkeeping. Every mutation happens
// while the caller holds that user's session lock (see Service.lockUser), so
// the read-modify-write the poller performs across a device round trip cannot
// interleave with a handler's trackMPDState. updateIfPresent additionally
// refuses to recreate a key that was deleted in the meantime, so a store can
// never resurrect an entry TransferPlayback removed on purpose.
type mpdTracker struct {
	mu      sync.Mutex
	entries map[int64]activeMPDSession
}

func (t *mpdTracker) set(userID int64, entry activeMPDSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.entries == nil {
		t.entries = make(map[int64]activeMPDSession)
	}
	t.entries[userID] = entry
}

// updateIfPresent writes entry only when the user still has an entry.
func (t *mpdTracker) updateIfPresent(userID int64, entry activeMPDSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.entries[userID]; !ok {
		return
	}
	t.entries[userID] = entry
}

func (t *mpdTracker) get(userID int64) (activeMPDSession, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.entries[userID]
	return entry, ok
}

func (t *mpdTracker) delete(userID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, userID)
}

func (t *mpdTracker) userIDs() []int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := make([]int64, 0, len(t.entries))
	for id := range t.entries {
		ids = append(ids, id)
	}
	return ids
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
		session.Current != nil &&
		!s.isBrowserDevice(session.DeviceID)
	if !active {
		s.mpdUsers.delete(session.UserID)
		return
	}

	next := activeMPDSession{
		deviceID:      session.DeviceID,
		expectedTrack: session.Current.TrackID,
	}
	if prev, ok := s.mpdUsers.get(session.UserID); ok &&
		prev.deviceID == next.deviceID &&
		prev.expectedTrack == next.expectedTrack {
		// Same user, device, and track as last time — preserve the
		// poller's accumulated observation. Without this, the persist
		// that fires every UpdatePosition would itself reset observedPlay
		// each tick, and the auto-advance gate would never close.
		next.lastSongID = prev.lastSongID
		next.observedPlay = prev.observedPlay
	}
	s.mpdUsers.set(session.UserID, next)
}

// browserDeviceLookup is implemented by resolvers that can tell a browser tab
// from an MPD device without doing any I/O.
type browserDeviceLookup interface {
	IsBrowserDevice(deviceID string) bool
}

// isBrowserDevice reports whether the device is a browser tab. Browser tabs
// push their own position over WebSocket and are never polled; tracking one
// costs a resolver round trip a second to learn nothing.
func (s *Service) isBrowserDevice(deviceID string) bool {
	lookup, ok := s.deviceResolver.(browserDeviceLookup)
	if !ok {
		return false
	}
	return lookup.IsBrowserDevice(deviceID)
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
//     This kills the audio-wz blast-loop where MPD lingers in stop
//     while loading the URL.
//     - observedPlay → real track-end. Pre-clear observedPlay to absorb
//     the inevitable extra stop tick during MPD's Clear+Add+Play, then
//     call Next(). The post-Next persistAndBroadcast hits trackMPDState
//     which seeds a fresh entry for the new track.
//  6. state="" (some MPD versions when fully idle) → conservative skip.
func (s *Service) PollMPDPositions() {
	for _, userID := range s.mpdUsers.userIDs() {
		s.pollUser(userID)
	}
}

// pollUser runs one poll tick for a single user. The user's session lock is
// held for the whole tick — device round trip included — so the poller's
// read-modify-write of both the session and the tracker entry cannot
// interleave with a handler's. Lock order is always user lock → device lock;
// device commands never call back into the service, so there is no cycle.
func (s *Service) pollUser(userID int64) {
	unlock := s.lockUser(userID)
	defer unlock()

	entry, ok := s.mpdUsers.get(userID)
	if !ok {
		return
	}

	device, err := s.resolveDevice(entry.deviceID)
	if err != nil {
		s.logger.Debug("poll: resolve device failed", "userID", userID, "error", err)
		return
	}
	status, err := device.Status()
	if err != nil {
		s.logger.Debug("poll: status failed", "userID", userID, "error", err)
		return
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
		s.mpdUsers.updateIfPresent(userID, entry)
		s.updatePositionLocked(userID, status.Elapsed, "", false)
	case "pause":
		s.mpdUsers.updateIfPresent(userID, entry)
	case "stop":
		if !entry.observedPlay {
			// Device hasn't started this song yet. Don't advance, don't
			// touch position. Just persist the (possibly updated)
			// lastSongID so a later songid change is still detectable.
			s.mpdUsers.updateIfPresent(userID, entry)
			s.logger.Debug("poll: stop without prior play; not advancing",
				"userID", userID, "deviceID", entry.deviceID)
			return
		}
		// Real track-end. Pre-clear observedPlay so the inevitable
		// extra stop tick that MPD reports during Clear+Add+Play
		// doesn't trigger a second Next(). The post-Next trackMPDState
		// will reseed for the new track.
		entry.observedPlay = false
		s.mpdUsers.updateIfPresent(userID, entry)
		if _, err := s.nextLocked(userID); err != nil {
			s.logger.Warn("poll: auto-advance failed", "userID", userID,
				"deviceID", entry.deviceID, "error", err)
		}
	default:
		// Empty or unknown state: be conservative, do nothing.
		s.mpdUsers.updateIfPresent(userID, entry)
	}
}
