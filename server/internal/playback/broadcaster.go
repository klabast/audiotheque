package playback

// SessionBroadcaster broadcasts session changes to a user's connected clients.
// Implementations typically push over WebSocket. exceptClientID is the ID of a
// client to skip (e.g. the browser that just sent the position update we're
// broadcasting); pass "" to send to every client of the user.
type SessionBroadcaster interface {
	BroadcastSession(userID int64, session *Session, exceptClientID string)
}

// SessionBroadcasterFunc adapts a function to the SessionBroadcaster interface.
type SessionBroadcasterFunc func(userID int64, session *Session, exceptClientID string)

// BroadcastSession calls the wrapped function.
func (f SessionBroadcasterFunc) BroadcastSession(userID int64, session *Session, exceptClientID string) {
	f(userID, session, exceptClientID)
}
