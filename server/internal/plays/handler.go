package plays

import (
	"encoding/json"
	"errors"
	"net/http"
)

type ServiceInterface interface {
	Stats(userID int64, period Period) (*Stats, error)
}

type UserIDGetter func(r *http.Request) (int64, error)

type Handler struct {
	service   ServiceInterface
	getUserID UserIDGetter
}

func NewHandler(service ServiceInterface, getUserID UserIDGetter) *Handler {
	return &Handler{service: service, getUserID: getUserID}
}

type StatsResponse struct {
	Listens          int                 `json:"listens"`
	ListeningSeconds int                 `json:"listeningSeconds"`
	TopTracks        []TopTrackResponse  `json:"topTracks"`
	TopAlbums        []TopAlbumResponse  `json:"topAlbums"`
	TopArtists       []TopArtistResponse `json:"topArtists"`
}

type TopTrackResponse struct {
	TrackID    int64  `json:"trackId"`
	Title      string `json:"title"`
	ArtistName string `json:"artistName"`
	AlbumID    int64  `json:"albumId"`
	AlbumTitle string `json:"albumTitle"`
	Plays      int    `json:"plays"`
}

type TopAlbumResponse struct {
	AlbumID    int64  `json:"albumId"`
	Title      string `json:"title"`
	ArtistName string `json:"artistName"`
	Plays      int    `json:"plays"`
}

type TopArtistResponse struct {
	ArtistID int64  `json:"artistId"`
	Name     string `json:"name"`
	Plays    int    `json:"plays"`
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stats", h.HandleStats)
}

// HandleStats handles GET /api/stats
// @Summary Listening stats
// @Description Listens, time listened and the most played tracks, albums and artists over a period
// @Tags stats
// @Produce json
// @Param period query string true "Period" Enums(week, month, year, all)
// @Success 200 {object} StatsResponse
// @Failure 400 {string} string "Unknown period"
// @Failure 401 {string} string "Unauthorized"
// @Router /stats [get]
// @ID getStats
func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	stats, err := h.service.Stats(userID, Period(r.URL.Query().Get("period")))
	if errors.Is(err, ErrUnknownPeriod) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(statsToResponse(stats)); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func statsToResponse(s *Stats) StatsResponse {
	resp := StatsResponse{
		Listens:          s.Listens,
		ListeningSeconds: s.ListeningSeconds,
		TopTracks:        make([]TopTrackResponse, 0, len(s.TopTracks)),
		TopAlbums:        make([]TopAlbumResponse, 0, len(s.TopAlbums)),
		TopArtists:       make([]TopArtistResponse, 0, len(s.TopArtists)),
	}
	for _, t := range s.TopTracks {
		resp.TopTracks = append(resp.TopTracks, TopTrackResponse(t))
	}
	for _, a := range s.TopAlbums {
		resp.TopAlbums = append(resp.TopAlbums, TopAlbumResponse(a))
	}
	for _, a := range s.TopArtists {
		resp.TopArtists = append(resp.TopArtists, TopArtistResponse(a))
	}
	return resp
}
