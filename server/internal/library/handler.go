package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"audiod/internal/auth"
)

// ServiceInterface defines the methods the handler depends on
type ServiceInterface interface {
	ListLibraries(userID int64) ([]*Library, error)
	CreateLibrary(userID int64, name string, paths []string) (*Library, error)
	StartScan(libraryID int64) error
	GetScanProgress(userID, libraryID int64) (*ScanProgress, error)
	DeleteLibrary(libraryID int64) error
	UpdateLibrary(libraryID int64, name string, paths []string) (*Library, error)
	ListAlbums(userID, libraryID int64, opts ListAlbumsOptions) ([]*AlbumWithArtist, error)
	GetAlbum(userID, albumID int64) (*AlbumWithArtist, error)
	GetAlbumCoverPath(userID, albumID int64) (string, error)
	ListTracksByAlbum(userID, albumID int64) ([]*TrackWithArtist, error)
	Search(userID, libraryID int64, query string) (*SearchResult, error)
}

type Handler struct {
	service     ServiceInterface
	authService *auth.Service
	thumbnailer *CoverThumbnailer
}

func NewHandler(service ServiceInterface, authService *auth.Service) *Handler {
	return &Handler{
		service:     service,
		authService: authService,
	}
}

// SetThumbnailer wires an optional cover thumbnailer. When set, requests with
// ?size=thumb (or other registered presets) get a scaled, on-disk-cached JPEG.
func (h *Handler) SetThumbnailer(t *CoverThumbnailer) {
	h.thumbnailer = t
}

// RegisterRoutes registers all library routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	user := func(next auth.AuthedHandlerFunc) http.HandlerFunc {
		return auth.RequireUser(h.authService, next)
	}
	admin := func(next auth.AuthedHandlerFunc) http.HandlerFunc {
		return auth.RequireAdminUser(h.authService, next)
	}

	// Library routes
	mux.HandleFunc("GET /api/libraries", user(h.HandleListLibraries))
	mux.HandleFunc("POST /api/libraries", admin(h.HandleCreateLibrary))
	mux.HandleFunc("PUT /api/libraries/{id}", admin(h.HandleUpdateLibrary))
	mux.HandleFunc("DELETE /api/libraries/{id}", admin(h.HandleDeleteLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan", admin(h.HandleScanLibrary))
	mux.HandleFunc("GET /api/libraries/{id}/scan-status", user(h.HandleGetScanStatus))
	mux.HandleFunc("GET /api/libraries/{id}/albums", user(h.HandleListAlbums))
	mux.HandleFunc("GET /api/libraries/{id}/search", user(h.HandleSearch))

	// Album routes
	mux.HandleFunc("GET /api/albums/{id}", user(h.HandleGetAlbum))
	mux.HandleFunc("GET /api/albums/{id}/cover", user(h.HandleGetAlbumCover))
	mux.HandleFunc("GET /api/albums/{id}/tracks", user(h.HandleListAlbumTracks))
}

// writeServiceError maps a service-layer error onto a response, keeping the
// distinction between "you may not" and "we broke" that a blanket 500 loses.
func writeServiceError(w http.ResponseWriter, err error, msg string) {
	switch {
	case errors.Is(err, ErrLibraryAccessDenied):
		http.Error(w, "Forbidden", http.StatusForbidden)
	case errors.Is(err, ErrAlbumNotFound):
		http.Error(w, "Album not found", http.StatusNotFound)
	case errors.Is(err, ErrNameRequired):
		http.Error(w, "Library name is required", http.StatusBadRequest)
	case errors.Is(err, ErrPathsRequired):
		http.Error(w, "At least one library path is required", http.StatusBadRequest)
	default:
		http.Error(w, msg, http.StatusInternalServerError)
	}
}

type CreateLibraryRequest struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

type UpdateLibraryRequest struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

type AlbumResponse struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	ArtistName   string `json:"artistName"`
	ReleaseDate  string `json:"releaseDate,omitempty"`
	Genre        string `json:"genre,omitempty"`
	TotalTracks  int    `json:"totalTracks"`
	CoverArtPath string `json:"coverArtPath,omitempty"`
	IsHiRes      bool   `json:"isHiRes"`
}

type LibraryResponse struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Paths      []string `json:"paths"`
	TrackCount int      `json:"trackCount"`
	AlbumCount int      `json:"albumCount"`
}

func libraryToResponse(lib *Library) LibraryResponse {
	return LibraryResponse{
		ID:         lib.ID,
		Name:       lib.Name,
		Paths:      lib.Paths,
		TrackCount: lib.TrackCount,
		AlbumCount: lib.AlbumCount,
	}
}

// HandleListLibraries handles GET /api/libraries
// @Summary List libraries
// @Description Get all libraries accessible to the authenticated user
// @Tags libraries
// @Produce json
// @Success 200 {array} LibraryResponse
// @Router /libraries [get]
// @ID listLibraries
func (h *Handler) HandleListLibraries(w http.ResponseWriter, r *http.Request, user *auth.User) {
	libraries, err := h.service.ListLibraries(user.ID)
	if err != nil {
		log.Printf("Failed to list libraries for user %d: %v", user.ID, err)
		http.Error(w, "Failed to list libraries", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	response := make([]LibraryResponse, len(libraries))
	for i, lib := range libraries {
		response[i] = libraryToResponse(lib)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// HandleCreateLibrary handles POST /api/libraries
// @Summary Create library
// @Description Create a new library (admin only)
// @Tags libraries
// @Accept json
// @Produce json
// @Param request body CreateLibraryRequest true "Library details"
// @Success 201 {object} LibraryResponse
// @Router /libraries [post]
// @ID createLibrary
func (h *Handler) HandleCreateLibrary(w http.ResponseWriter, r *http.Request, user *auth.User) {
	var req CreateLibraryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	library, err := h.service.CreateLibrary(user.ID, req.Name, req.Paths)
	if err != nil {
		log.Printf("Failed to create library for user %d: %v", user.ID, err)
		writeServiceError(w, err, "Failed to create library")
		return
	}

	response := libraryToResponse(library)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// HandleScanLibrary handles POST /api/libraries/:id/scan
// @Summary Start library scan
// @Description Start scanning a library for new/modified audio files
// @Tags libraries
// @Produce json
// @Param id path int true "Library ID"
// @Success 202 {object} map[string]string "Scan started"
// @Failure 409 {object} map[string]string "Scan already in progress"
// @Router /libraries/{id}/scan [post]
// @ID scanLibrary
func (h *Handler) HandleScanLibrary(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Extract library ID from URL path parameter
	libraryIDStr := r.PathValue("id")
	libraryID, err := parseLibraryID(libraryIDStr)
	if err != nil {
		http.Error(w, "Invalid library ID", http.StatusBadRequest)
		return
	}

	err = h.service.StartScan(libraryID)
	if err == ErrScanAlreadyInProgress {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict) // 409
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Scan already in progress",
		})
		return
	}

	if err != nil {
		log.Printf("Failed to start scan for library %d: %v", libraryID, err)
		http.Error(w, "Failed to start scan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Scan started",
	})
}

// HandleGetScanStatus handles GET /api/libraries/:id/scan-status
// @Summary Get scan progress
// @Description Get current scan progress for a library
// @Tags libraries
// @Produce json
// @Param id path int true "Library ID"
// @Success 200 {object} ScanProgress "Scan in progress"
// @Success 204 "No scan in progress"
// @Router /libraries/{id}/scan-status [get]
// @ID getScanStatus
func (h *Handler) HandleGetScanStatus(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Extract library ID from URL path parameter
	libraryIDStr := r.PathValue("id")
	libraryID, err := parseLibraryID(libraryIDStr)
	if err != nil {
		http.Error(w, "Invalid library ID", http.StatusBadRequest)
		return
	}

	progress, err := h.service.GetScanProgress(user.ID, libraryID)
	if errors.Is(err, ErrNoScanInProgress) {
		w.WriteHeader(http.StatusNoContent) // 204
		return
	}

	if err != nil {
		log.Printf("Failed to get scan status for library %d: %v", libraryID, err)
		writeServiceError(w, err, "Failed to get scan status")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(progress); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// parseLibraryID parses library ID from string
// parseSortQuery turns "album-artist:asc,year:desc" into []SortSpec.
// Unknown fields and directions are silently dropped so the caller still gets
// the service-level default when nothing valid was provided.
func parseSortQuery(s string) []SortSpec {
	if s == "" {
		return nil
	}
	var specs []SortSpec
	for _, part := range strings.Split(s, ",") {
		field := strings.TrimSpace(part)
		dir := SortAsc
		if i := strings.Index(part, ":"); i >= 0 {
			field = strings.TrimSpace(part[:i])
			d := strings.ToLower(strings.TrimSpace(part[i+1:]))
			if d == "desc" {
				dir = SortDesc
			}
		}
		if !ValidSortFields[SortField(field)] {
			continue
		}
		specs = append(specs, SortSpec{Field: SortField(field), Direction: dir})
	}
	return specs
}

func parseLibraryID(idStr string) (int64, error) {
	if idStr == "" {
		return 0, ErrLibraryNotFound
	}
	// For now, simple conversion - can be enhanced with strconv.ParseInt
	var id int64
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, ErrLibraryNotFound
	}
	return id, nil
}

// HandleDeleteLibrary handles DELETE /api/libraries/:id
// @Summary Delete library
// @Description Delete a library and all associated data (admin only)
// @Tags libraries
// @Param id path int true "Library ID"
// @Success 204 "Library deleted"
// @Failure 403 {string} string "Forbidden: admin access required"
// @Failure 404 {string} string "Library not found"
// @Router /libraries/{id} [delete]
// @ID deleteLibrary
func (h *Handler) HandleDeleteLibrary(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Extract library ID from URL path parameter
	libraryIDStr := r.PathValue("id")
	libraryID, err := parseLibraryID(libraryIDStr)
	if err != nil {
		http.Error(w, "Invalid library ID", http.StatusBadRequest)
		return
	}

	err = h.service.DeleteLibrary(libraryID)
	if errors.Is(err, ErrLibraryNotFound) {
		http.Error(w, "Library not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Failed to delete library %d: %v", libraryID, err)
		http.Error(w, "Failed to delete library", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleUpdateLibrary handles PUT /api/libraries/:id
// @Summary Update library
// @Description Update a library's name and paths (admin only)
// @Tags libraries
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Param request body UpdateLibraryRequest true "Library details"
// @Success 200 {object} LibraryResponse
// @Failure 403 {string} string "Forbidden: admin access required"
// @Failure 404 {string} string "Library not found"
// @Router /libraries/{id} [put]
// @ID updateLibrary
func (h *Handler) HandleUpdateLibrary(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Extract library ID from URL path parameter
	libraryIDStr := r.PathValue("id")
	libraryID, err := parseLibraryID(libraryIDStr)
	if err != nil {
		http.Error(w, "Invalid library ID", http.StatusBadRequest)
		return
	}

	var req UpdateLibraryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	library, err := h.service.UpdateLibrary(libraryID, req.Name, req.Paths)
	if errors.Is(err, ErrLibraryNotFound) {
		http.Error(w, "Library not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Failed to update library %d: %v", libraryID, err)
		writeServiceError(w, err, "Failed to update library")
		return
	}

	response := libraryToResponse(library)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// HandleListAlbums handles GET /api/libraries/:id/albums
// @Summary List albums in a library
// @Description Get all albums in a library accessible to the authenticated user
// @Tags libraries
// @Produce json
// @Param id path int true "Library ID"
// @Param hiRes query bool false "Filter to albums containing at least one hi-res track"
// @Param sort query string false "Sort spec: comma-separated `<field>:<asc|desc>` levels. Fields: album-artist, artist, album-title, year"
// @Success 200 {array} AlbumResponse
// @Router /libraries/{id}/albums [get]
// @ID listAlbums
func (h *Handler) HandleListAlbums(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Extract library ID from URL path parameter
	libraryIDStr := r.PathValue("id")
	libraryID, err := parseLibraryID(libraryIDStr)
	if err != nil {
		http.Error(w, "Invalid library ID", http.StatusBadRequest)
		return
	}

	opts := ListAlbumsOptions{
		HiResOnly: r.URL.Query().Get("hiRes") == "true",
		SortBy:    parseSortQuery(r.URL.Query().Get("sort")),
	}

	albums, err := h.service.ListAlbums(user.ID, libraryID, opts)
	if err != nil {
		log.Printf("Failed to list albums for library %d: %v", libraryID, err)
		writeServiceError(w, err, "Failed to list albums")
		return
	}

	// Convert to response format
	response := make([]AlbumResponse, len(albums))
	for i, a := range albums {
		response[i] = AlbumResponse{
			ID:           a.Album.ID,
			Title:        a.Album.Title,
			ArtistName:   a.ArtistName,
			ReleaseDate:  a.Album.ReleaseDate,
			Genre:        a.Album.Genre,
			TotalTracks:  a.Album.TotalTracks,
			CoverArtPath: a.Album.CoverArtPath,
			IsHiRes:      a.Album.IsHiRes,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// HandleGetAlbumCover handles GET /api/albums/{id}/cover
// @Summary Get album cover art
// @Description Get the cover art image for an album
// @Tags albums
// @Produce image/jpeg,image/png
// @Param id path int true "Album ID"
// @Success 200 {file} binary "Cover art image"
// @Failure 404 {object} map[string]string "Album not found or no cover"
// @Router /albums/{id}/cover [get]
// @ID getAlbumCover
func (h *Handler) HandleGetAlbumCover(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Parse album ID from URL
	albumIDStr := r.PathValue("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid album ID", http.StatusBadRequest)
		return
	}

	// Get cover path from service
	coverPath, err := h.service.GetAlbumCoverPath(user.ID, albumID)
	if err != nil {
		log.Printf("Failed to get album cover path: %v", err)
		writeServiceError(w, err, "Internal server error")
		return
	}

	// No cover art
	if coverPath == "" {
		http.Error(w, "No cover art", http.StatusNotFound)
		return
	}

	// Optional thumbnail variant: ?size=thumb. Falls back to the original on
	// any unknown value. The thumbnailer caches rendered output keyed by the
	// source mtime so a re-import busts the cache for free.
	servePath := coverPath
	contentTypeOverride := ""
	if size := r.URL.Query().Get("size"); size != "" && h.thumbnailer != nil {
		if preset, ok := LookupCoverSize(size); ok {
			info, statErr := os.Stat(coverPath)
			if statErr == nil && !info.IsDir() {
				thumbPath, err := h.thumbnailer.Resolve(coverPath, preset, info.ModTime().UnixNano())
				if err == nil {
					servePath = thumbPath
					contentTypeOverride = "image/jpeg"
				} else {
					log.Printf("cover thumbnail render failed for %d: %v", albumID, err)
				}
			}
		}
	}

	// Open and serve the file
	file, err := os.Open(servePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Cover file not found", http.StatusNotFound)
			return
		}
		log.Printf("Failed to open cover file: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Determine content type from extension (or override for rendered thumbs)
	contentType := "image/jpeg" // default
	if contentTypeOverride != "" {
		contentType = contentTypeOverride
	} else if strings.HasSuffix(strings.ToLower(servePath), ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(strings.ToLower(servePath), ".gif") {
		contentType = "image/gif"
	} else if strings.HasSuffix(strings.ToLower(servePath), ".webp") {
		contentType = "image/webp"
	}

	// Get file info for size
	stat, err := file.Stat()
	if err != nil {
		log.Printf("Failed to stat cover file: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	// Thumbnails are cache-keyed by source mtime — safe to cache for a year.
	// Originals get a shorter window because their cache key is just the URL.
	if contentTypeOverride == "image/jpeg" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}

	http.ServeContent(w, r, servePath, stat.ModTime(), file)
}

type TrackResponse struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	ArtistName  string `json:"artistName"`
	TrackNumber int    `json:"trackNumber"`
	DiscNumber  int    `json:"discNumber"`
	Duration    int    `json:"duration"`
	IsLossless  bool   `json:"isLossless"`
	IsHiRes     bool   `json:"isHiRes"`
}

// HandleGetAlbum handles GET /api/albums/:id
// @Summary Get album details
// @Description Get details for a specific album
// @Tags albums
// @Produce json
// @Param id path int true "Album ID"
// @Success 200 {object} AlbumResponse
// @Failure 404 {string} string "Album not found"
// @Router /albums/{id} [get]
// @ID getAlbum
func (h *Handler) HandleGetAlbum(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Parse album ID from URL
	albumIDStr := r.PathValue("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid album ID", http.StatusBadRequest)
		return
	}

	album, err := h.service.GetAlbum(user.ID, albumID)
	if err != nil {
		log.Printf("Failed to get album: %v", err)
		writeServiceError(w, err, "Internal server error")
		return
	}

	response := AlbumResponse{
		ID:           album.Album.ID,
		Title:        album.Album.Title,
		ArtistName:   album.ArtistName,
		ReleaseDate:  album.Album.ReleaseDate,
		Genre:        album.Album.Genre,
		TotalTracks:  album.Album.TotalTracks,
		CoverArtPath: album.Album.CoverArtPath,
		IsHiRes:      album.Album.IsHiRes,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// HandleListAlbumTracks handles GET /api/albums/:id/tracks
// @Summary List album tracks
// @Description Get all tracks in an album
// @Tags albums
// @Produce json
// @Param id path int true "Album ID"
// @Success 200 {array} TrackResponse
// @Failure 404 {string} string "Album not found"
// @Router /albums/{id}/tracks [get]
// @ID listAlbumTracks
func (h *Handler) HandleListAlbumTracks(w http.ResponseWriter, r *http.Request, user *auth.User) {
	// Parse album ID from URL
	albumIDStr := r.PathValue("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid album ID", http.StatusBadRequest)
		return
	}

	tracks, err := h.service.ListTracksByAlbum(user.ID, albumID)
	if err != nil {
		log.Printf("Failed to list tracks: %v", err)
		writeServiceError(w, err, "Internal server error")
		return
	}

	// Convert to response format
	response := make([]TrackResponse, len(tracks))
	for i, t := range tracks {
		response[i] = TrackResponse{
			ID:          t.Track.ID,
			Title:       t.Track.Title,
			ArtistName:  t.ArtistName,
			TrackNumber: t.Track.TrackNumber,
			DiscNumber:  t.Track.DiscNumber,
			Duration:    t.Track.Duration,
			IsLossless:  t.Track.IsLossless,
			IsHiRes:     t.Track.IsHiRes,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

type SearchAlbumResult struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	IsHiRes bool   `json:"isHiRes"`
}

type SearchArtistResult struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SearchTrackResult struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	AlbumID *int64 `json:"albumId,omitempty"`
}

type SearchResponse struct {
	Albums  []SearchAlbumResult  `json:"albums"`
	Artists []SearchArtistResult `json:"artists"`
	Tracks  []SearchTrackResult  `json:"tracks"`
}

// HandleSearch handles GET /api/libraries/{id}/search
// @Summary Search a library
// @Description Full-text search for albums, artists, and tracks in a library, ranked by relevance. Matches title/name plus artist, genre and year; accent- and case-insensitive with prefix matching.
// @Tags libraries
// @Produce json
// @Param id path int true "Library ID"
// @Param q query string true "Search query"
// @Success 200 {object} SearchResponse
// @Router /libraries/{id}/search [get]
// @ID searchLibrary
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request, user *auth.User) {
	libraryIDStr := r.PathValue("id")
	libraryID, err := parseLibraryID(libraryIDStr)
	if err != nil {
		http.Error(w, "Invalid library ID", http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))

	result, err := h.service.Search(user.ID, libraryID, query)
	if err != nil {
		log.Printf("Search failed for library %d, query %q: %v", libraryID, query, err)
		writeServiceError(w, err, "Search failed")
		return
	}

	response := SearchResponse{
		Albums:  make([]SearchAlbumResult, len(result.Albums)),
		Artists: make([]SearchArtistResult, len(result.Artists)),
		Tracks:  make([]SearchTrackResult, len(result.Tracks)),
	}
	for i, a := range result.Albums {
		response.Albums[i] = SearchAlbumResult{
			ID:      a.Album.ID,
			Title:   a.Album.Title,
			Artist:  a.ArtistName,
			IsHiRes: a.Album.IsHiRes,
		}
	}
	for i, a := range result.Artists {
		response.Artists[i] = SearchArtistResult{ID: a.ID, Name: a.Name}
	}
	for i, t := range result.Tracks {
		response.Tracks[i] = SearchTrackResult{
			ID:      t.Track.ID,
			Title:   t.Track.Title,
			Artist:  t.ArtistName,
			AlbumID: t.Track.AlbumID,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
