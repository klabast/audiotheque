package playback

import "audiod/internal/library"

// LibraryTrackProvider adapts the library service to provide tracks for playback
type LibraryTrackProvider struct {
	libraryService *library.Service
}

// NewLibraryTrackProvider creates a new track provider
func NewLibraryTrackProvider(libraryService *library.Service) *LibraryTrackProvider {
	return &LibraryTrackProvider{libraryService: libraryService}
}

// GetAlbumTracks returns tracks for an album the user can access.
func (p *LibraryTrackProvider) GetAlbumTracks(userID, albumID int64) ([]Track, error) {
	tracksWithArtist, err := p.libraryService.ListTracksByAlbum(userID, albumID)
	if err != nil {
		return nil, err
	}

	tracks := make([]Track, len(tracksWithArtist))
	for i, twa := range tracksWithArtist {
		artistID := int64(0)
		if twa.Track.ArtistID != nil {
			artistID = *twa.Track.ArtistID
		}
		albumID := int64(0)
		if twa.Track.AlbumID != nil {
			albumID = *twa.Track.AlbumID
		}
		tracks[i] = Track{
			ID:       twa.Track.ID,
			Title:    twa.Track.Title,
			AlbumID:  albumID,
			ArtistID: artistID,
			Duration: twa.Track.Duration,
		}
	}
	return tracks, nil
}
