// Package main Audiotheque Music Server API
//
// Audiotheque is a self-hosted music streaming server with hi-res audio support.
//
//	Schemes: http, https
//	BasePath: /api
//	Version: 1.0.0
//	Host: localhost:8080
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//	- text/event-stream
//	- audio/flac
//	- audio/mpeg
//	- audio/ogg
//
// swagger:meta
package main

// Common error response
// swagger:response errorResponse
type errorResponseWrapper struct {
	// in:body
	Body struct {
		// Error message
		// example: Invalid request
		Error string `json:"error"`
	}
}

// swagger:model Album
type Album struct {
	// Album ID
	// example: 1
	ID int64 `json:"id"`

	// Artist name
	// example: Pink Floyd
	Artist string `json:"artist"`

	// Artist ID
	// example: 42
	ArtistID int64 `json:"artistId"`

	// Album artist name (may differ from track artist)
	// example: Pink Floyd
	AlbumArtist string `json:"albumArtist,omitempty"`

	// Album artist ID
	// example: 42
	AlbumArtistID *int64 `json:"albumArtistId,omitempty"`

	// Album name
	// example: The Dark Side of the Moon
	AlbumName string `json:"albumName"`

	// Genre
	// example: Progressive Rock
	Genre string `json:"genre,omitempty"`

	// Release year
	// example: 1973
	Year int `json:"year,omitempty"`

	// Number of tracks
	// example: 10
	TrackCount int `json:"trackCount"`

	// Whether album contains hi-res audio
	// example: true
	IsHiRes bool `json:"isHiRes"`

	// Whether album has cover art
	// example: true
	HasCover bool `json:"hasCover"`

	// Cover art URL
	// example: /api/covers/1
	CoverURL string `json:"coverUrl,omitempty"`
}

// swagger:model Track
type Track struct {
	// Track ID
	// example: 123
	ID int64 `json:"id"`

	// Title
	// example: Time
	Title string `json:"title"`

	// Artist name
	// example: Pink Floyd
	Artist string `json:"artist"`

	// Album name
	// example: The Dark Side of the Moon
	Album string `json:"album"`

	// Album ID
	// example: 1
	AlbumID int64 `json:"albumId"`

	// Track number
	// example: 4
	TrackNumber int `json:"trackNumber"`

	// Disc number
	// example: 1
	DiscNumber int `json:"discNumber"`

	// Duration in seconds
	// example: 413
	Duration int `json:"duration"`

	// File path
	// example: /music/Pink Floyd/The Dark Side of the Moon/04 - Time.flac
	FilePath string `json:"filePath"`

	// Audio format
	// example: flac
	Format string `json:"format"`

	// Bit rate
	// example: 1411
	Bitrate int `json:"bitrate"`

	// Sample rate
	// example: 44100
	SampleRate int `json:"sampleRate"`

	// Whether track is hi-res
	// example: false
	IsHiRes bool `json:"isHiRes"`
}

// swagger:model Library
type Library struct {
	// Library ID
	// example: 1
	ID int64 `json:"id"`

	// Library name
	// example: Main Library
	Name string `json:"name"`

	// Library paths
	// example: ["/music"]
	Paths []string `json:"paths"`

	// Whether library is active
	// example: true
	IsActive bool `json:"isActive"`

	// Last scan timestamp
	// example: 2024-01-10T15:30:00Z
	LastScanAt *string `json:"lastScanAt,omitempty"`

	// Number of tracks
	// example: 5432
	TrackCount int `json:"trackCount"`

	// Number of albums
	// example: 234
	AlbumCount int `json:"albumCount"`

	// Number of artists
	// example: 89
	ArtistCount int `json:"artistCount"`
}

// swagger:model PlaybackStatus
type PlaybackStatus struct {
	// Whether playback is active
	// example: true
	IsPlaying bool `json:"isPlaying"`

	// Current position in seconds
	// example: 125
	Position int `json:"position"`

	// Total duration in seconds
	// example: 413
	Duration int `json:"duration"`

	// Volume level (0-100)
	// example: 75
	Volume int `json:"volume"`

	// Current track
	CurrentTrack *Track `json:"currentTrack,omitempty"`
}

// swagger:model TranscodeFormat
type TranscodeFormat struct {
	// Format name
	// example: MP3
	Name string `json:"name"`

	// Format identifier
	// example: mp3
	Format string `json:"format"`

	// MIME type
	// example: audio/mpeg
	MimeType string `json:"mimeType"`

	// Available quality options
	// example: ["128", "192", "320"]
	Qualities []string `json:"qualities"`
}
