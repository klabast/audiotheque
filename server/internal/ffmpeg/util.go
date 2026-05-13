package ffmpeg

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// GetFFmpegPath returns the path to the FFmpeg binary
func GetFFmpegPath() string {
	// First check if it's in system PATH
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path
	}

	// Check custom locations
	customPath := filepath.Join("bin", runtime.GOOS+"-"+runtime.GOARCH, "ffmpeg")
	if _, err := os.Stat(customPath); err == nil {
		return customPath
	}

	// Try relative to executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		possiblePath := filepath.Join(execDir, "bin", runtime.GOOS+"-"+runtime.GOARCH, "ffmpeg")
		if _, err := os.Stat(possiblePath); err == nil {
			return possiblePath
		}
	}

	// Not found
	return ""
}

// TestFFprobe tests if a given ffprobe path actually works
func TestFFprobe(path string) bool {
	cmd := exec.Command(path, "-version")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		log.Printf("ffprobe test failed: %v", err)
		return false
	}

	return true
}

// GetFFprobePath returns the path to the FFprobe binary
// This tries to find ffprobe and ensures it actually works
func GetFFprobePath() string {
	// First check if it's in system PATH
	if path, err := exec.LookPath("ffprobe"); err == nil {
		if TestFFprobe(path) {
			log.Printf("Found ffprobe in PATH: %s", path)
			return path
		}
		log.Printf("Found ffprobe in PATH but it doesn't work: %s", path)
	}

	// Try to use ffmpeg binary with -version argument
	ffmpegPath := GetFFmpegPath()
	if ffmpegPath == "" {
		log.Printf("Cannot look for ffprobe: ffmpeg path not found")
		return ""
	}

	log.Printf("Looking for ffprobe alongside ffmpeg at: %s", filepath.Dir(ffmpegPath))

	// Check for ffprobe in ffmpeg directory
	ffmpegDir := filepath.Dir(ffmpegPath)
	ffprobePath := filepath.Join(ffmpegDir, "ffprobe")
	if runtime.GOOS == "windows" {
		ffprobePath += ".exe"
	}

	if _, err := os.Stat(ffprobePath); err == nil {
		log.Printf("Found ffprobe at: %s", ffprobePath)

		// Check if it works
		if TestFFprobe(ffprobePath) {
			log.Printf("Confirmed ffprobe works at: %s", ffprobePath)
			return ffprobePath
		}

		log.Printf("ffprobe exists at %s but doesn't work, attempting to fix permissions", ffprobePath)
		os.Chmod(ffprobePath, 0755) // Make sure it's executable

		if TestFFprobe(ffprobePath) {
			log.Printf("Fixed permissions, ffprobe now works at: %s", ffprobePath)
			return ffprobePath
		}

		log.Printf("ffprobe still doesn't work after fixing permissions: %s", ffprobePath)
	} else {
		log.Printf("ffprobe not found at expected location: %s", ffprobePath)
	}

	// Also check bin directory
	binDir := "./bin"
	if _, err := os.Stat(binDir); err == nil {
		// Look for ffprobe in platform-specific directory
		platformDir := filepath.Join(binDir, runtime.GOOS+"-"+runtime.GOARCH)
		ffprobePath = filepath.Join(platformDir, "ffprobe")
		if runtime.GOOS == "windows" {
			ffprobePath += ".exe"
		}

		if _, err := os.Stat(ffprobePath); err == nil {
			log.Printf("Found ffprobe in bin directory: %s", ffprobePath)
			if TestFFprobe(ffprobePath) {
				return ffprobePath
			}
		}
	}

	// ffprobe not found or doesn't work
	log.Printf("No working ffprobe found")
	return ""
}

// EnsureFFprobe ensures ffprobe is downloaded using the downloader
func EnsureFFprobe() string {
	// First try to find an existing one
	path := GetFFprobePath()
	if path != "" {
		return path
	}

	// If not found, download it
	downloader := NewDownloader("./bin")
	path, err := downloader.EnsureFFprobe()
	if err != nil {
		log.Printf("Failed to download ffprobe: %v", err)
		return ""
	}

	return path
}
