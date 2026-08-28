package playback

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// TrackProvider provides access to tracks (from library)
type TrackProvider interface {
	GetAlbumTracks(userID, albumID int64) ([]Track, error)
}

// SessionRepository handles session persistence. There is at most one
// session per user (Spotify model). Delete is used when a session's device
// disappears (orphan cleanup) and by the startup migration that purges
// pre-invariant rows.
type SessionRepository interface {
	Save(session *Session) error
	GetByUserID(userID int64) (*Session, error)
	Delete(userID int64) error
	// DeleteWithoutDevice removes any persisted session whose DeviceID is
	// empty — those rows pre-date the unified-device invariant and have no
	// way to resume playback. Returns the number of rows removed.
	DeleteWithoutDevice() (int64, error)
}

// ErrNoDevice signals that an operation requires an active playback device
// but the session doesn't name one. The handler maps this to 409 Conflict.
var ErrNoDevice = errors.New("no playback device assigned to session")

// ErrNoSession signals that the user has no active playback session. The
// handler maps this to 404 Not Found.
var ErrNoSession = errors.New("no active session")

// DeviceResolver resolves a device ID to a PlaybackDevice. Empty deviceID is
// not a valid input under the unified-device invariant — callers must
// short-circuit before reaching the resolver.
type DeviceResolver interface {
	ResolveDevice(deviceID string) (PlaybackDevice, error)
}

// StreamURLBuilder creates a streaming URL for a track ID. The userID is
// included so the wiring layer can mint a user-scoped signed token for MPD
// devices (which fetch the URL without a session cookie).
type StreamURLBuilder func(trackID, userID int64) string

// Service handles playback session management
type Service struct {
	tracks         TrackProvider
	sessions       SessionRepository
	deviceResolver DeviceResolver
	streamURL      StreamURLBuilder
	broadcaster    SessionBroadcaster
	logger         *slog.Logger

	// mpdUsers tracks which users currently have a playing session on an MPD
	// device. The poller reads this; request handlers write to it via
	// trackMPDState in persistAndBroadcast.
	mpdUsers mpdTracker

	// userLocks serializes each user's session read-modify-write. Every
	// service entry point takes its user's lock and holds it across the whole
	// read → mutate → save, device round trip included: three writers (the 1 Hz
	// poller, one readPump per browser tab, HTTP handlers) otherwise interleave
	// and drop each other's updates.
	//
	// Lock ordering is always user lock → device lock. Device commands never
	// call back into the service, so the reverse edge doesn't exist and the
	// ordering cannot cycle. Holding the lock across MPD I/O means one user's
	// requests queue behind their own device; the bound comes from
	// MPDPlaybackDevice's command timeout.
	userLocks sync.Map // userID -> *sync.Mutex
}

// lockUser acquires the user's session lock and returns its release function.
func (s *Service) lockUser(userID int64) func() {
	v, _ := s.userLocks.LoadOrStore(userID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// NewService creates a new playback service
func NewService(tracks TrackProvider, sessions SessionRepository) *Service {
	return &Service{
		tracks:   tracks,
		sessions: sessions,
		logger:   slog.Default(),
	}
}

func (s *Service) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

func (s *Service) SetDeviceResolver(resolver DeviceResolver) {
	s.deviceResolver = resolver
}

func (s *Service) SetStreamURLBuilder(builder StreamURLBuilder) {
	s.streamURL = builder
}

func (s *Service) SetBroadcaster(b SessionBroadcaster) {
	s.broadcaster = b
}

// persistAndBroadcast saves the session, updates poller bookkeeping, and
// pushes the session to every connected client of the user. Broadcast only
// happens if the save succeeds, so clients never see state that the server
// failed to persist.
func (s *Service) persistAndBroadcast(session *Session) error {
	return s.persistAndBroadcastExcept(session, "")
}

// persistAndBroadcastExcept is like persistAndBroadcast but skips the given
// client ID — used when the change originated from that client (e.g. a
// playback-position tick from the browser) so we don't echo it back.
func (s *Service) persistAndBroadcastExcept(session *Session, exceptClientID string) error {
	if err := s.sessions.Save(session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	s.trackMPDState(session)
	if s.broadcaster != nil {
		s.broadcaster.BroadcastSession(session.UserID, session, exceptClientID)
	}
	return nil
}

// PlayAlbumOnDevice starts playback of an album on a specific device. The
// deviceID is REQUIRED: a session has no meaning without an owning device
// under the unified-session invariant. Handlers translate "no body deviceId"
// into the requesting tab's hub client ID before reaching this layer.
func (s *Service) PlayAlbumOnDevice(userID, albumID, startTrackID int64, deviceID string) (*Session, error) {
	if deviceID == "" {
		return nil, ErrNoDevice
	}
	unlock := s.lockUser(userID)
	defer unlock()

	previous, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	tracks, err := s.tracks.GetAlbumTracks(userID, albumID)
	if err != nil {
		return nil, fmt.Errorf("failed to get album tracks: %w", err)
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("album has no tracks")
	}

	// Find starting index. 0 means "first track".
	startIdx := 0
	if startTrackID != 0 {
		startIdx = -1
		for i, t := range tracks {
			if t.ID == startTrackID {
				startIdx = i
				break
			}
		}
		if startIdx < 0 {
			return nil, fmt.Errorf("startTrackID %d not in album %d", startTrackID, albumID)
		}
	}

	remaining := make([]int64, 0, len(tracks)-startIdx-1)
	for i := startIdx + 1; i < len(tracks); i++ {
		remaining = append(remaining, tracks[i].ID)
	}

	session := &Session{
		UserID:   userID,
		State:    StatePlaying,
		DeviceID: deviceID,
		Current: &CurrentTrack{
			TrackID:  tracks[startIdx].ID,
			Position: 0,
		},
		Queue: []QueueItem{},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        albumID,
			Remaining: remaining,
		},
		History:   []int64{},
		IsPrivate: false,
	}

	// Forward to device BEFORE persisting so we don't broadcast a "playing"
	// state that the device hasn't been asked to honor yet. A device that
	// refuses the play must surface that to the caller: treating it as
	// success leaves the session naming a device that isn't playing.
	if err := s.playOnDevice(session); err != nil {
		return nil, fmt.Errorf("play on device: %w", err)
	}

	// Only one device may hold the session, so the one it just left has to be
	// told to stop — otherwise both keep playing and the old one, no longer
	// named by any session, can never be reached again. Stopping after the new
	// device accepted the play keeps a failed play from silencing everything.
	s.stopPreviousDevice(previous, deviceID)

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// stopPreviousDevice stops the device the session was playing on when playback
// moves elsewhere. Best-effort: the old device may have disconnected, which is
// exactly the case where there is nothing left to stop.
func (s *Service) stopPreviousDevice(previous *Session, newDeviceID string) {
	if previous == nil || previous.DeviceID == "" || previous.DeviceID == newDeviceID {
		return
	}
	if previous.State != StatePlaying && previous.State != StatePaused {
		return
	}
	device, err := s.resolveDevice(previous.DeviceID)
	if err != nil {
		s.logger.Warn("resolve previous device to stop it failed",
			"deviceID", previous.DeviceID, "error", err)
		return
	}
	if err := device.Stop(); err != nil {
		s.logger.Warn("stop previous device failed",
			"deviceID", previous.DeviceID, "error", err)
	}
}

// GetSession retrieves a user's current playback session. If the session is
// bound to a device that no longer resolves (a stale browser-tab clientID
// from a previous WS connection, or an MPD device that was removed), the
// session is deleted entirely — under the unified-device invariant an
// orphaned session has no owner and cannot be controlled. The caller sees
// (nil, nil), same as a user who never started playback. They can press
// play on any client to create a fresh session bound to that client.
func (s *Service) GetSession(userID int64) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil || session == nil {
		return session, err
	}
	if session.DeviceID == "" {
		// Pre-invariant row that survived migration somehow. Treat as orphan.
		s.logger.Debug("deleting session with empty deviceID", "userID", userID)
		s.deleteOrphan(userID)
		return nil, nil
	}
	if s.deviceResolver == nil {
		return session, nil
	}
	if _, err := s.deviceResolver.ResolveDevice(session.DeviceID); err != nil {
		// Only a device that is genuinely gone orphans the session. A refused
		// dial, a timeout, an MPD box mid-reboot — those are transient, and
		// deleting on them costs the user their queue, position and history
		// on the next page load.
		if !errors.Is(err, ErrDeviceNotFound) {
			s.logger.Warn("session device is unreachable; keeping the session",
				"userID", userID, "deviceID", session.DeviceID, "error", err)
			return session, nil
		}
		s.logger.Info("deleting session whose device disappeared",
			"userID", userID, "deviceID", session.DeviceID, "error", err)
		s.deleteOrphan(userID)
		return nil, nil
	}
	return session, nil
}

// deleteOrphan removes a session with no addressable owner, poller bookkeeping
// included — a leftover entry keeps the poller resolving a dead device once a
// second forever.
func (s *Service) deleteOrphan(userID int64) {
	s.mpdUsers.delete(userID)
	if err := s.sessions.Delete(userID); err != nil {
		s.logger.Warn("delete orphaned session failed", "userID", userID, "error", err)
	}
}

// Pause pauses playback and saves the current position. If the session is
// bound to a device and the device rejects the pause command, the error is
// returned and the session is NOT persisted as paused — clients must see the
// real device state, not an optimistic lie.
func (s *Service) Pause(userID int64, position int) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}

	device, err := s.resolveDevice(session.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("resolve device for pause: %w", err)
	}
	if err := device.Pause(); err != nil {
		return nil, fmt.Errorf("device pause: %w", err)
	}

	session.State = StatePaused
	if session.Current != nil {
		session.Current.Position = position
	}

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// Resume resumes playback from the current position. Like Pause, device
// failures abort the resume so the persisted state never disagrees with what
// the device is actually doing.
func (s *Service) Resume(userID int64) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}

	device, err := s.resolveDevice(session.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("resolve device for resume: %w", err)
	}
	if err := device.Resume(); err != nil {
		return nil, fmt.Errorf("device resume: %w", err)
	}

	session.State = StatePlaying

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// Next advances to the next track
// Priority: explicit queue first, then source remaining
func (s *Service) Next(userID int64) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()
	return s.nextLocked(userID)
}

// nextLocked implements Next; callers must hold the user's session lock.
func (s *Service) nextLocked(userID int64) (*Session, error) {
	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}

	// Add current track to history (if exists)
	if session.Current != nil {
		session.History = append(session.History, session.Current.TrackID)
	}

	// Try to get next track from explicit queue first
	if len(session.Queue) > 0 {
		nextTrack := session.Queue[0]
		session.Queue = session.Queue[1:] // Remove from queue
		session.Current = &CurrentTrack{
			TrackID:  nextTrack.TrackID,
			Position: 0,
		}
	} else if len(session.Source.Remaining) > 0 {
		// Get next from source remaining
		nextTrackID := session.Source.Remaining[0]
		session.Source.Remaining = session.Source.Remaining[1:]
		session.Current = &CurrentTrack{
			TrackID:  nextTrackID,
			Position: 0,
		}
	} else {
		// No more tracks - stop playback. The device has to be told: setting
		// state=stopped on our side alone leaves MPD happily playing on.
		session.State = StateStopped
		session.Current = nil
		s.stopDevice(session.DeviceID)
	}

	// Forward to device when there's still a track to play and the session
	// is in playing state.
	if session.Current != nil && session.State == StatePlaying {
		if err := s.playOnDevice(session); err != nil {
			return nil, s.persistFailedAdvance(session, "next", err)
		}
	}

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// stopDevice tells the session's device to stop. Best-effort — a device we
// can't reach has nothing we can do about it.
func (s *Service) stopDevice(deviceID string) {
	if deviceID == "" {
		return
	}
	device, err := s.resolveDevice(deviceID)
	if err != nil {
		s.logger.Warn("resolve device to stop it failed", "deviceID", deviceID, "error", err)
		return
	}
	if err := device.Stop(); err != nil {
		s.logger.Warn("stop device failed", "deviceID", deviceID, "error", err)
	}
}

// persistFailedAdvance saves the track change the device refused, marked
// paused, and returns the wrapped error.
//
// The persist is load-bearing, not bookkeeping. The poller pre-clears
// observedPlay before calling Next, so a session left at the old track with
// state=playing is never advanced again — every later tick reads "stop without
// prior play" and skips. Persisting a paused session at the new track drops
// the poller entry (trackMPDState) and leaves the user one press of play away
// from recovering.
func (s *Service) persistFailedAdvance(session *Session, op string, cause error) error {
	s.logger.Warn("device refused the track change; parking the session as paused",
		"userID", session.UserID, "deviceID", session.DeviceID, "op", op, "error", cause)
	session.State = StatePaused
	if err := s.persistAndBroadcast(session); err != nil {
		s.logger.Error("persist after failed track change failed",
			"userID", session.UserID, "error", err)
	}
	return fmt.Errorf("play %s on device: %w", op, cause)
}

// Previous goes back to the previous track
// If no history, restarts current track
func (s *Service) Previous(userID int64) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}

	if len(session.History) > 0 {
		// Get previous track from history
		prevTrackID := session.History[len(session.History)-1]
		session.History = session.History[:len(session.History)-1]

		// Put current track back at front of remaining
		if session.Current != nil {
			session.Source.Remaining = append([]int64{session.Current.TrackID}, session.Source.Remaining...)
		}

		session.Current = &CurrentTrack{
			TrackID:  prevTrackID,
			Position: 0,
		}
	} else {
		// No history - just restart current track
		if session.Current != nil {
			session.Current.Position = 0
		}
	}

	// Forward to device only when the session is actually playing — same
	// guard as Next. Starting the device on a paused session left the two
	// disagreeing, and the poller ignores non-playing sessions, so the
	// position then never moved.
	if session.Current != nil && session.State == StatePlaying {
		if err := s.playOnDevice(session); err != nil {
			return nil, s.persistFailedAdvance(session, "previous", err)
		}
	}

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// SeekTrack moves the playback head to the given position (seconds) in the
// current track. Device failures abort. Named `SeekTrack` rather than `Seek`
// to avoid colliding with the `io.Seeker` standard-method signature — `go
// vet -stdmethods` flags any method called `Seek` whose first two args are
// `(int64, int)` because that almost-but-not-quite matches io.Seeker.
func (s *Service) SeekTrack(userID int64, position int) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}

	device, err := s.resolveDevice(session.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("resolve device for seek: %w", err)
	}
	if err := device.Seek(position); err != nil {
		return nil, fmt.Errorf("device seek: %w", err)
	}

	if session.Current != nil {
		session.Current.Position = position
	}

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// SetVolume sets the playback volume (0-100) and stores it per-device. Device
// failures abort so the per-device map and the actual hardware don't diverge.
func (s *Service) SetVolume(userID int64, volume int) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}

	device, err := s.resolveDevice(session.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("resolve device for volume: %w", err)
	}
	if err := device.SetVolume(volume); err != nil {
		// A mixerless device (e.g. MPD with `mixer_type "none"`) cannot
		// physically change its volume. The user-facing API still succeeds:
		// we persist the desired value so a future transfer to a device with
		// volume honors it, and surface the limitation via deviceCapabilities
		// in SessionResponse.
		if !errors.Is(err, ErrVolumeNotSupported) {
			return nil, fmt.Errorf("device set volume: %w", err)
		}
		s.logger.Info("device does not support volume; persisting intent only",
			"deviceID", session.DeviceID, "volume", volume)
	}

	if session.DeviceVolumes == nil {
		session.DeviceVolumes = make(map[string]int)
	}
	session.DeviceVolumes[session.DeviceID] = volume

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// UpdatePosition updates the current playback position and broadcasts to all
// of the user's clients. Used by server-side sources like the MPD poller,
// where the device itself is authoritative for elapsed time and the value
// can legitimately drop (track transitions, device-driven seeks).
func (s *Service) UpdatePosition(userID int64, position int) {
	unlock := s.lockUser(userID)
	defer unlock()
	s.updatePositionLocked(userID, position, "", false)
}

// UpdatePositionFromClient is the client-driven counterpart: a browser tab
// pushing its audio.currentTime over WS. Excludes the sending client from
// the broadcast and ignores regressions on the same track — those are
// stale `timeupdate` events racing with a server-driven seek and would
// clobber the seeked position with a pre-broadcast localCurrentTime.
func (s *Service) UpdatePositionFromClient(userID int64, position int, senderClientID string) {
	unlock := s.lockUser(userID)
	defer unlock()
	s.updatePositionLocked(userID, position, senderClientID, true)
}

// updatePositionLocked implements both position paths; callers must hold the
// user's session lock.
func (s *Service) updatePositionLocked(userID int64, position int, exceptClientID string, fromClient bool) {
	session, err := s.sessions.GetByUserID(userID)
	if err != nil || session == nil || session.Current == nil {
		return
	}
	if fromClient && position < session.Current.Position {
		// Stale browser push racing with a recent server-driven Seek/Next/etc.
		// Track transitions don't reach here (Service.Next sets position via
		// its own path), so a regression on the same track is always a race.
		return
	}
	session.Current.Position = position
	if err := s.persistAndBroadcastExcept(session, exceptClientID); err != nil {
		s.logger.Error("persist position failed", "userID", userID, "error", err)
	}
}

// TransferPlayback moves playback from the current device to a different one.
// Saves the old device's volume and position, restores the new device's
// volume if known. targetDeviceID must name a real device — the unified
// invariant rules out "transfer to nothing."
func (s *Service) TransferPlayback(userID int64, targetDeviceID string) (*Session, error) {
	unlock := s.lockUser(userID)
	defer unlock()

	if targetDeviceID == "" {
		return nil, ErrNoDevice
	}
	session, err := s.sessions.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrNoSession
	}

	if session.DeviceVolumes == nil {
		session.DeviceVolumes = make(map[string]int)
	}

	// Save state from old device before stopping. Every active session names
	// an old device under the invariant — but we still tolerate
	// resolve/Stop failures (the device may have just disconnected).
	// Remove the user from the MPD poller BEFORE stopping the device.
	// Otherwise the poller can observe state="stop" between Stop() and the
	// persistAndBroadcast at the end of this function, mistake it for
	// end-of-track, and call Next() — which races the transfer and resets
	// session.position to 0.
	s.mpdUsers.delete(userID)

	if oldDevice, err := s.resolveDevice(session.DeviceID); err != nil {
		s.logger.Error("resolve old device failed for transfer", "deviceID", session.DeviceID, "error", err)
	} else {
		if status, err := oldDevice.Status(); err == nil {
			if session.Current != nil {
				session.Current.Position = status.Elapsed
			}
			session.DeviceVolumes[session.DeviceID] = status.Volume
		}
		if err := oldDevice.Stop(); err != nil {
			s.logger.Error("stop old device failed for transfer", "deviceID", session.DeviceID, "error", err)
		}
	}

	oldDeviceID := session.DeviceID
	session.DeviceID = targetDeviceID
	s.logger.Debug("transferring playback",
		"from", oldDeviceID, "to", targetDeviceID)

	// Start playing on new device. If this fails (device unreachable, MPD
	// down, etc.) we MUST roll back DeviceID and surface the error — leaving
	// the session pointing at a broken device produces a silent failure
	// where the UI thinks playback moved but no audio is happening.
	if session.Current != nil && session.State == StatePlaying {
		if err := s.playOnDevice(session); err != nil {
			return nil, s.rollbackTransfer(session, oldDeviceID, err)
		}
	} else {
		// Nothing to start, but the target still has to exist: without this
		// check a paused session would be bound to any string the caller sent
		// and transferring to an offline device answered 200.
		if _, err := s.resolveDevice(targetDeviceID); err != nil {
			return nil, s.rollbackTransfer(session, oldDeviceID, err)
		}
	}

	// Restore volume on new device if we have a saved value. Best-effort:
	// a mixerless device just needs a debug breadcrumb; any other failure
	// is unexpected so we keep the Error log.
	if vol, ok := session.DeviceVolumes[targetDeviceID]; ok {
		if device, err := s.resolveDevice(targetDeviceID); err == nil {
			if err := device.SetVolume(vol); err != nil {
				if errors.Is(err, ErrVolumeNotSupported) {
					s.logger.Debug("skipped volume restore on mixerless device",
						"deviceID", targetDeviceID, "volume", vol)
				} else {
					s.logger.Error("restore volume failed",
						"deviceID", targetDeviceID, "error", err)
				}
			}
		}
	}

	if err := s.persistAndBroadcast(session); err != nil {
		return nil, err
	}

	return session, nil
}

// DeviceCapabilities reports what the named device can do, for the
// deviceCapabilities hint on SessionResponse. Returns nil when the device
// can't be resolved so the hint is omitted rather than guessed.
func (s *Service) DeviceCapabilities(deviceID string) *DeviceCapabilities {
	device, err := s.resolveDevice(deviceID)
	if err != nil {
		return nil
	}
	return &DeviceCapabilities{Volume: device.SupportsVolume()}
}

// rollbackTransfer puts the session back on the device it came from after a
// failed transfer. The old device was already stopped and its poller entry
// deleted, so restoring DeviceID alone leaves playback dead on both ends:
// restart it, and persist so trackMPDState re-seeds the poller. If the old
// device won't start either, park the session as paused rather than claiming
// to play on a device that isn't.
func (s *Service) rollbackTransfer(session *Session, oldDeviceID string, cause error) error {
	session.DeviceID = oldDeviceID
	if session.Current != nil && session.State == StatePlaying {
		if err := s.playOnDevice(session); err != nil {
			s.logger.Warn("restart of the old device after a failed transfer failed",
				"deviceID", oldDeviceID, "error", err)
			session.State = StatePaused
		}
	}
	if err := s.persistAndBroadcast(session); err != nil {
		s.logger.Error("persist after failed transfer failed",
			"userID", session.UserID, "error", err)
	}
	return fmt.Errorf("transfer failed: %w", cause)
}

// resolveDevice resolves a device ID to a PlaybackDevice
func (s *Service) resolveDevice(deviceID string) (PlaybackDevice, error) {
	if s.deviceResolver == nil {
		return nil, fmt.Errorf("no device resolver configured")
	}
	return s.deviceResolver.ResolveDevice(deviceID)
}

// playOnDevice sends play command for the current track to the session's device
func (s *Service) playOnDevice(session *Session) error {
	if session.Current == nil || s.streamURL == nil {
		return fmt.Errorf("missing track or stream URL builder")
	}
	device, err := s.resolveDevice(session.DeviceID)
	if err != nil {
		s.logger.Error("resolve device failed", "deviceID", session.DeviceID, "error", err)
		return err
	}
	url := s.streamURL(session.Current.TrackID, session.UserID)
	s.logger.Debug("playing on device", "deviceID", session.DeviceID, "url", url)
	if err := device.Play(url, session.Current.Position); err != nil {
		s.logger.Error("device play failed", "deviceID", session.DeviceID, "error", err)
		return err
	}
	return nil
}
