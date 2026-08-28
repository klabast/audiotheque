package library

import (
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
)

// AudioMetadata contains all extracted metadata from an audio file
type AudioMetadata struct {
	// Tag metadata (from dhowden/tag)
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	TrackNumber int
	TrackTotal  int
	DiscNumber  int
	DiscTotal   int
	Year        int
	Genre       string

	// MusicBrainz IDs (extracted from raw tags)
	MusicBrainzTrackID  string
	MusicBrainzAlbumID  string
	MusicBrainzArtistID string

	// Cover art
	HasPicture  bool
	PictureData []byte
	PictureMIME string

	// Audio properties (filled by FFprobe)
	Duration   int // milliseconds
	SampleRate int // Hz
	BitDepth   int // bits (16, 24, 32)
	Channels   int
	Codec      string // flac, mp3, aac, etc.
	Bitrate    int    // kbps (lossy only)

	// Computed
	IsLossless bool
	IsHiRes    bool // bit_depth >= 24 OR sample_rate >= 48000
}

// ExtractMetadata reads metadata from an audio file using tag library + FFprobe
func ExtractMetadata(filePath string) (*AudioMetadata, error) {
	meta := &AudioMetadata{}

	// Extract tag metadata
	if err := extractTags(filePath, meta); err != nil {
		// Don't fail completely - we can still try FFprobe
		// Use filename as title fallback
		meta.Title = filepath.Base(filePath)
	}

	// Extract audio properties via FFprobe
	if err := extractAudioProperties(filePath, meta); err != nil {
		// Log but don't fail - tag metadata is still valid
		// Set some defaults
		meta.Codec = codecFromExtension(filePath)
	}

	// Compute derived fields
	meta.computeDerivedFields()

	return meta, nil
}

// extractTags uses dhowden/tag to read metadata tags
func extractTags(filePath string, meta *AudioMetadata) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return err
	}

	// Basic metadata
	meta.Title = m.Title()
	meta.Artist = m.Artist()
	meta.Album = m.Album()
	meta.AlbumArtist = m.AlbumArtist()
	meta.Year = m.Year()
	meta.Genre = m.Genre()

	// Track/disc numbers
	track, trackTotal := m.Track()
	meta.TrackNumber = track
	meta.TrackTotal = trackTotal

	disc, discTotal := m.Disc()
	meta.DiscNumber = disc
	meta.DiscTotal = discTotal
	if meta.DiscNumber == 0 {
		meta.DiscNumber = 1 // Default to disc 1
	}

	// MusicBrainz IDs from raw tags
	if raw := m.Raw(); raw != nil {
		meta.MusicBrainzTrackID = getStringTag(raw, "musicbrainz_trackid", "MUSICBRAINZ_TRACKID", "MusicBrainz Track Id")
		meta.MusicBrainzAlbumID = getStringTag(raw, "musicbrainz_albumid", "MUSICBRAINZ_ALBUMID", "MusicBrainz Album Id")
		meta.MusicBrainzArtistID = getStringTag(raw, "musicbrainz_artistid", "MUSICBRAINZ_ARTISTID", "MusicBrainz Artist Id")
	}

	// Cover art
	if pic := m.Picture(); pic != nil {
		meta.HasPicture = true
		meta.PictureData = pic.Data
		meta.PictureMIME = pic.MIMEType
	}

	return nil
}

// getStringTag tries multiple tag names to find a value
func getStringTag(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := raw[key]; ok {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// computeDerivedFields calculates IsLossless and IsHiRes
func (m *AudioMetadata) computeDerivedFields() {
	// Determine if lossless based on codec
	losslessCodecs := map[string]bool{
		"flac": true,
		"alac": true,
		"wav":  true,
		"aiff": true,
		"pcm":  true,
		"dsd":  true,
		"dsf":  true,
	}
	m.IsLossless = losslessCodecs[m.Codec]

	// Hi-res only applies to lossless formats
	// Requirements: lossless AND (bit depth >= 24 OR sample rate >= 48000)
	m.IsHiRes = m.IsLossless && (m.BitDepth >= 24 || m.SampleRate >= 48000)

	// For lossy formats, bit depth from sample_fmt is not meaningful (decoder internal)
	// Reset to 0 for lossy formats
	if !m.IsLossless {
		m.BitDepth = 0
	}
}

// codecFromExtension returns codec name based on file extension (fallback)
func codecFromExtension(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".flac":
		return "flac"
	case ".mp3":
		return "mp3"
	case ".m4a", ".aac":
		return "aac"
	case ".ogg":
		return "ogg"
	case ".wav":
		return "wav"
	case ".aiff", ".aif":
		return "aiff"
	case ".dsf":
		return "dsf"
	default:
		return "unknown"
	}
}
