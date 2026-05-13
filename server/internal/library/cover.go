package library

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExtractCoverArt saves embedded artwork to disk
// Returns the path relative to dataDir, or empty string if no art
func ExtractCoverArt(meta *AudioMetadata, albumID int64, dataDir string) (string, error) {
	if !meta.HasPicture || len(meta.PictureData) == 0 {
		return "", nil
	}

	// Determine file extension from MIME type
	ext := extensionFromMIME(meta.PictureMIME)

	// Create covers directory
	coversDir := filepath.Join(dataDir, "covers")
	if err := os.MkdirAll(coversDir, 0755); err != nil {
		return "", fmt.Errorf("create covers directory: %w", err)
	}

	// Write cover file
	filename := fmt.Sprintf("album_%d%s", albumID, ext)
	fullPath := filepath.Join(coversDir, filename)

	if err := os.WriteFile(fullPath, meta.PictureData, 0644); err != nil {
		return "", fmt.Errorf("write cover file: %w", err)
	}

	// Return relative path
	return filepath.Join("covers", filename), nil
}

// extensionFromMIME returns file extension for a MIME type
func extensionFromMIME(mime string) string {
	switch mime {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg" // Default to jpg
	}
}
