package playback

import (
	"encoding/json"
	"net/http"
)

// AddToQueueRequest queues either tracks or a whole album, never both.
type AddToQueueRequest struct {
	TrackIDs []int64 `json:"trackIds,omitempty"`
	AlbumID  int64   `json:"albumId,omitempty"`
	// Position is "next" (up next) or "end" (after everything queued).
	Position string `json:"position" enums:"next,end"`
}

// MoveQueueItemRequest moves a queue entry to a zero-based index. Indexes
// past either end land on that end.
type MoveQueueItemRequest struct {
	Index int `json:"index"`
}

// HandleAddToQueue handles POST /api/playback/queue
// @Summary Add to queue
// @Description Queue tracks or an album, up next or at the end
// @Tags playback
// @Accept json
// @Produce json
// @Param request body AddToQueueRequest true "What to queue and where"
// @Success 200 {object} SessionResponse
// @Failure 400 {string} string "Bad request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session, or track or album not found"
// @Router /playback/queue [post]
// @ID addToQueue
func (h *Handler) HandleAddToQueue(w http.ResponseWriter, r *http.Request) {
	var req AddToQueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	position := QueuePosition(req.Position)
	if position != QueueNext && position != QueueEnd {
		http.Error(w, `position must be "next" or "end"`, http.StatusBadRequest)
		return
	}
	if (len(req.TrackIDs) == 0) == (req.AlbumID == 0) {
		http.Error(w, "Specify either trackIds or albumId", http.StatusBadRequest)
		return
	}

	h.respondWithSession(w, r, func(userID int64) (*Session, error) {
		if req.AlbumID != 0 {
			return h.service.AddAlbumToQueue(userID, req.AlbumID, position)
		}
		return h.service.AddTracksToQueue(userID, req.TrackIDs, position)
	})
}

// HandleClearQueue handles DELETE /api/playback/queue
// @Summary Clear queue
// @Description Empty the queue; what is playing and the source carry on
// @Tags playback
// @Produce json
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session"
// @Router /playback/queue [delete]
// @ID clearQueue
func (h *Handler) HandleClearQueue(w http.ResponseWriter, r *http.Request) {
	h.respondWithSession(w, r, h.service.ClearQueue)
}

// HandleRemoveFromQueue handles DELETE /api/playback/queue/{itemId}
// @Summary Remove from queue
// @Tags playback
// @Produce json
// @Param itemId path string true "Queue item ID"
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or queue item"
// @Router /playback/queue/{itemId} [delete]
// @ID removeFromQueue
func (h *Handler) HandleRemoveFromQueue(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	h.respondWithSession(w, r, func(userID int64) (*Session, error) {
		return h.service.RemoveFromQueue(userID, itemID)
	})
}

// HandleMoveQueueItem handles POST /api/playback/queue/{itemId}/move
// @Summary Move a queue entry
// @Tags playback
// @Accept json
// @Produce json
// @Param itemId path string true "Queue item ID"
// @Param request body MoveQueueItemRequest true "Target index"
// @Success 200 {object} SessionResponse
// @Failure 400 {string} string "Bad request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or queue item"
// @Router /playback/queue/{itemId}/move [post]
// @ID moveQueueItem
func (h *Handler) HandleMoveQueueItem(w http.ResponseWriter, r *http.Request) {
	var req MoveQueueItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	itemID := r.PathValue("itemId")
	h.respondWithSession(w, r, func(userID int64) (*Session, error) {
		return h.service.MoveQueueItem(userID, itemID, req.Index)
	})
}

// HandlePlayFromQueue handles POST /api/playback/queue/{itemId}/play
// @Summary Play a queue entry now
// @Description Jump to a queued track; entries queued before it are dropped
// @Tags playback
// @Produce json
// @Param itemId path string true "Queue item ID"
// @Success 200 {object} SessionResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active session or queue item"
// @Failure 502 {string} string "Playback device unreachable"
// @Router /playback/queue/{itemId}/play [post]
// @ID playFromQueue
func (h *Handler) HandlePlayFromQueue(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	h.respondWithSession(w, r, func(userID int64) (*Session, error) {
		return h.service.PlayFromQueue(userID, itemID)
	})
}

func (h *Handler) respondWithSession(w http.ResponseWriter, r *http.Request, op func(userID int64) (*Session, error)) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, err := op(userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionToResponse(session, h.capabilitiesFor())); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
