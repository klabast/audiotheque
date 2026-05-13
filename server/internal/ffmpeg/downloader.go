package ffmpeg

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DownloadURLProvider returns download URLs for FFmpeg binaries based on OS and architecture
type DownloadURLProvider interface {
	GetFFmpegURL(os, arch string) (string, error)
	GetFFprobeURL(os, arch string) (string, error)
}

// DefaultURLProvider implements the standard download URLs for FFmpeg
type DefaultURLProvider struct{}

// GetFFmpegURL returns the appropriate download URL for FFmpeg on the current platform
func (p *DefaultURLProvider) GetFFmpegURL(os, arch string) (string, error) {
	switch os {
	case "darwin":
		if arch == "arm64" {
			// Apple Silicon (M1/M2/M3)
			return "https://www.osxexperts.net/ffmpeg711arm.zip", nil
		} else {
			// Intel Mac
			return "https://www.osxexperts.net/ffmpeg71intel.zip", nil
		}
	case "linux":
		if arch == "amd64" {
			return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz", nil
		} else if arch == "arm64" {
			return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz", nil
		}
	case "windows":
		// Windows - using the essential build which is smaller
		return "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", nil
	}

	return "", fmt.Errorf("no download URL available for %s/%s", os, arch)
}

// GetFFprobeURL returns the appropriate download URL for FFprobe on the current platform
func (p *DefaultURLProvider) GetFFprobeURL(os, arch string) (string, error) {
	switch os {
	case "darwin":
		if arch == "arm64" {
			// Apple Silicon (M1/M2/M3)
			return "https://www.osxexperts.net/ffprobe711arm.zip", nil
		} else {
			// Intel Mac
			return "https://www.osxexperts.net/ffprobe71intel.zip", nil
		}
	case "linux":
		if arch == "amd64" {
			// For Linux, we'll use the same package as FFmpeg
			return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz", nil
		} else if arch == "arm64" {
			return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz", nil
		}
	case "windows":
		// Windows - using the same package as FFmpeg
		return "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", nil
	}

	return "", fmt.Errorf("no download URL available for %s/%s", os, arch)
}

// Downloader handles the downloading and extraction of FFmpeg binaries
type Downloader struct {
	BinDir      string
	URLProvider DownloadURLProvider
	client      *http.Client
}

// NewDownloader creates a new FFmpeg downloader
func NewDownloader(binDir string) *Downloader {
	if binDir == "" {
		binDir = "./bin"
	}

	return &Downloader{
		BinDir:      binDir,
		URLProvider: &DefaultURLProvider{},
		client:      &http.Client{Timeout: 5 * time.Minute}, // Increased timeout to 5 minutes
	}
}

// EnsureFFmpeg ensures that FFmpeg is available, downloading it if necessary
func (d *Downloader) EnsureFFmpeg() (string, error) {
	// Check if we already have FFmpeg for this platform
	ffmpegPath, err := d.findExistingFFmpeg()
	if err == nil {
		// FFmpeg already exists
		log.Printf("Using existing FFmpeg at %s", ffmpegPath)
		return ffmpegPath, nil
	}

	// Create the binary directory if it doesn't exist
	binPath := filepath.Join(d.BinDir, runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.MkdirAll(binPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create binary directory: %v", err)
	}

	// Get the download URL for this platform
	url, err := d.URLProvider.GetFFmpegURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	// Download FFmpeg
	log.Printf("Downloading FFmpeg from %s...", url)
	ffmpegPath = filepath.Join(binPath, "ffmpeg")
	if runtime.GOOS == "windows" {
		ffmpegPath += ".exe"
	}

	// Download to a temporary file
	tmpFile := ffmpegPath + ".tmp"
	if err := d.downloadFile(url, tmpFile); err != nil {
		// If download fails, try to use system FFmpeg as a last resort
		systemPath, sysErr := exec.LookPath("ffmpeg")
		if sysErr == nil {
			log.Printf("Download failed, but found system FFmpeg at %s", systemPath)
			return systemPath, nil
		}
		return "", fmt.Errorf("failed to download FFmpeg: %v", err)
	}

	// Extract if needed
	if strings.HasSuffix(url, ".zip") || strings.HasSuffix(url, ".tar.xz") {
		log.Printf("Extracting FFmpeg binary...")
		if err := d.extractFFmpeg(tmpFile, ffmpegPath); err != nil {
			return "", fmt.Errorf("failed to extract FFmpeg: %v", err)
		}
	} else {
		// Rename the downloaded file if it's a direct binary
		if err := os.Rename(tmpFile, ffmpegPath); err != nil {
			return "", fmt.Errorf("failed to rename FFmpeg binary: %v", err)
		}
	}

	// Make the binary executable
	if err := os.Chmod(ffmpegPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make FFmpeg executable: %v", err)
	}

	log.Printf("FFmpeg installed at %s", ffmpegPath)
	return ffmpegPath, nil
}

// EnsureFFprobe ensures that FFprobe is available, downloading it if necessary
func (d *Downloader) EnsureFFprobe() (string, error) {
	// Check if we already have FFprobe for this platform
	ffprobePath, err := d.findExistingFFprobe()
	if err == nil {
		// FFprobe already exists
		log.Printf("Using existing FFprobe at %s", ffprobePath)
		return ffprobePath, nil
	}

	// Create the binary directory if it doesn't exist
	binPath := filepath.Join(d.BinDir, runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.MkdirAll(binPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create binary directory: %v", err)
	}

	// Get the download URL for this platform
	url, err := d.URLProvider.GetFFprobeURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	// Download FFprobe
	log.Printf("Downloading FFprobe from %s...", url)
	ffprobePath = filepath.Join(binPath, "ffprobe")
	if runtime.GOOS == "windows" {
		ffprobePath += ".exe"
	}

	// Download to a temporary file
	tmpFile := ffprobePath + ".tmp"
	if err := d.downloadFile(url, tmpFile); err != nil {
		// If download fails, try to use system FFprobe as a last resort
		systemPath, sysErr := exec.LookPath("ffprobe")
		if sysErr == nil {
			log.Printf("Download failed, but found system FFprobe at %s", systemPath)
			return systemPath, nil
		}
		return "", fmt.Errorf("failed to download FFprobe: %v", err)
	}

	// Extract if needed
	if strings.HasSuffix(url, ".zip") || strings.HasSuffix(url, ".tar.xz") {
		log.Printf("Extracting FFprobe binary...")
		if err := d.extractFFprobe(tmpFile, ffprobePath); err != nil {
			return "", fmt.Errorf("failed to extract FFprobe: %v", err)
		}
	} else {
		// Rename the downloaded file if it's a direct binary
		if err := os.Rename(tmpFile, ffprobePath); err != nil {
			return "", fmt.Errorf("failed to rename FFprobe binary: %v", err)
		}
	}

	// Make the binary executable
	if err := os.Chmod(ffprobePath, 0755); err != nil {
		return "", fmt.Errorf("failed to make FFprobe executable: %v", err)
	}

	log.Printf("FFprobe installed at %s", ffprobePath)
	return ffprobePath, nil
}

// findExistingFFmpeg checks if FFmpeg is already installed
func (d *Downloader) findExistingFFmpeg() (string, error) {
	// First check for system FFmpeg
	systemPath, err := exec.LookPath("ffmpeg")
	if err == nil {
		log.Printf("Found system FFmpeg at %s", systemPath)
		return systemPath, nil
	}

	// Check for bundled FFmpeg
	binPath := filepath.Join(d.BinDir, runtime.GOOS+"-"+runtime.GOARCH)
	ffmpegName := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegName += ".exe"
	}

	bundledPath := filepath.Join(binPath, ffmpegName)
	if _, err := os.Stat(bundledPath); err == nil {
		// Ensure it's executable
		os.Chmod(bundledPath, 0755)
		return bundledPath, nil
	}

	return "", fmt.Errorf("FFmpeg not found")
}

// findExistingFFprobe checks if FFprobe is already installed
func (d *Downloader) findExistingFFprobe() (string, error) {
	// First check for system FFprobe
	systemPath, err := exec.LookPath("ffprobe")
	if err == nil {
		log.Printf("Found system FFprobe at %s", systemPath)
		return systemPath, nil
	}

	// Check for bundled FFprobe
	binPath := filepath.Join(d.BinDir, runtime.GOOS+"-"+runtime.GOARCH)
	ffprobeName := "ffprobe"
	if runtime.GOOS == "windows" {
		ffprobeName += ".exe"
	}

	bundledPath := filepath.Join(binPath, ffprobeName)
	if _, err := os.Stat(bundledPath); err == nil {
		// Ensure it's executable
		os.Chmod(bundledPath, 0755)
		return bundledPath, nil
	}

	return "", fmt.Errorf("FFprobe not found")
}

// downloadFile downloads a file from a URL to a local path
func (d *Downloader) downloadFile(url, destPath string) error {
	// Create the destination file
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the file
	resp, err := d.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	log.Printf("Download completed to %s", destPath)
	return nil
}

// extractFFmpeg extracts the FFmpeg binary from an archive
func (d *Downloader) extractFFmpeg(archivePath, destPath string) error {
	log.Printf("Extracting archive %s using system commands", archivePath)

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "ffmpeg-extract")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use system unzip command - much more reliable
	cmd := exec.Command("unzip", "-j", archivePath, "-d", tempDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Unzip output: %s", string(output))
		return fmt.Errorf("failed to unzip: %v", err)
	}

	// Find the ffmpeg binary in the extracted files
	var ffmpegFile string
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp dir: %v", err)
	}

	log.Printf("Files extracted from zip:")
	for _, file := range files {
		log.Printf("  %s", file.Name())
		if strings.Contains(strings.ToLower(file.Name()), "ffmpeg") {
			ffmpegFile = file.Name()
		}
	}

	if ffmpegFile == "" {
		// Just use the first file if we can't find a clear match
		if len(files) > 0 {
			ffmpegFile = files[0].Name()
		} else {
			return fmt.Errorf("no files found in extracted archive")
		}
	}

	// Copy the file to the destination
	srcPath := filepath.Join(tempDir, ffmpegFile)
	log.Printf("Moving %s to %s", srcPath, destPath)

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	return nil
}

// extractFFprobe extracts the FFprobe binary from an archive
func (d *Downloader) extractFFprobe(archivePath, destPath string) error {
	log.Printf("Extracting archive %s using system commands", archivePath)

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "ffprobe-extract")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use system unzip command - much more reliable
	cmd := exec.Command("unzip", "-j", archivePath, "-d", tempDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Unzip output: %s", string(output))
		return fmt.Errorf("failed to unzip: %v", err)
	}

	// Find the ffprobe binary in the extracted files
	var ffprobeFile string
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp dir: %v", err)
	}

	log.Printf("Files extracted from zip:")
	for _, file := range files {
		log.Printf("  %s", file.Name())
		if strings.Contains(strings.ToLower(file.Name()), "ffprobe") {
			ffprobeFile = file.Name()
		}
	}

	if ffprobeFile == "" {
		// Just use the first file if we can't find a clear match
		if len(files) > 0 {
			ffprobeFile = files[0].Name()
		} else {
			return fmt.Errorf("no files found in extracted archive")
		}
	}

	// Copy the file to the destination
	srcPath := filepath.Join(tempDir, ffprobeFile)
	log.Printf("Moving %s to %s", srcPath, destPath)

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	return nil
}
