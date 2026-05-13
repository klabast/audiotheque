package ffmpeg

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ExtractFFmpegBinary extracts the FFmpeg binary from an archive
func ExtractFFmpegBinary(archivePath, extractDir string) (string, error) {
	// Create extraction directory if it doesn't exist
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create extraction directory: %v", err)
	}

	// Handle different archive formats
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath, extractDir)
	} else if strings.HasSuffix(archivePath, ".tar.xz") {
		return extractFromTarXz(archivePath, extractDir)
	}

	// If it's not an archive, assume it's a direct binary
	destPath := filepath.Join(extractDir, "ffmpeg")
	if runtime.GOOS == "windows" {
		destPath += ".exe"
	}

	if err := os.Rename(archivePath, destPath); err != nil {
		return "", fmt.Errorf("failed to move binary: %v", err)
	}

	return destPath, nil
}

// extractFromZip extracts FFmpeg from a zip archive
func extractFromZip(zipPath, extractDir string) (string, error) {
	log.Printf("Extracting zip file: %s to directory: %s", zipPath, extractDir)

	// Open the zip file
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip file: %v", err)
	}
	defer r.Close()

	// First, let's list all files in the zip for debugging
	log.Printf("Files in zip archive:")
	for _, f := range r.File {
		log.Printf("  %s (size: %d, is dir: %v)", f.Name, f.UncompressedSize64, f.FileInfo().IsDir())
	}

	// Create a temporary directory to extract all files
	tempDir := filepath.Join(extractDir, "temp_extract")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp extraction directory: %v", err)
	}

	// Extract all files from the zip
	for _, f := range r.File {
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Create directory for this file if needed
		destPath := filepath.Join(tempDir, f.Name)
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %v", destDir, err)
		}

		// Extract the file
		log.Printf("Extracting: %s to %s", f.Name, destPath)

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open file in zip: %v", err)
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("failed to create output file: %v", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("failed to copy file content: %v", err)
		}
	}

	// Now look for the ffmpeg binary in the extracted files
	var ffmpegPath string
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if this file looks like the ffmpeg binary
		name := filepath.Base(path)

		// First, look for exact matches
		if name == "ffmpeg" || name == "ffmpeg.exe" {
			ffmpegPath = path
			return io.EOF // Stop walking
		}

		// Then, check for files that contain "ffmpeg" in the name
		if strings.Contains(strings.ToLower(name), "ffmpeg") && info.Mode()&0111 != 0 {
			// It's an executable with ffmpeg in the name
			ffmpegPath = path
			return io.EOF // Stop walking
		}

		return nil
	})

	if err != nil && err != io.EOF {
		return "", fmt.Errorf("error while searching for ffmpeg: %v", err)
	}

	if ffmpegPath == "" {
		// If we didn't find a clear match, just look for any executable
		err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && info.Mode()&0111 != 0 {
				// It's an executable
				ffmpegPath = path
				return io.EOF // Stop walking
			}

			return nil
		})

		if err != nil && err != io.EOF {
			return "", fmt.Errorf("error while searching for executables: %v", err)
		}
	}

	if ffmpegPath == "" {
		return "", fmt.Errorf("ffmpeg binary not found in zip archive")
	}

	// Move the ffmpeg binary to the final location
	destPath := filepath.Join(extractDir, "ffmpeg")
	if runtime.GOOS == "windows" {
		destPath += ".exe"
	}

	log.Printf("Moving ffmpeg from %s to %s", ffmpegPath, destPath)

	if err := os.Rename(ffmpegPath, destPath); err != nil {
		return "", fmt.Errorf("failed to move ffmpeg binary: %v", err)
	}

	// Ensure it's executable
	if err := os.Chmod(destPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make ffmpeg executable: %v", err)
	}

	// Clean up the temp directory
	os.RemoveAll(tempDir)

	return destPath, nil
}

// extractFromTarXz extracts FFmpeg from a tar.xz archive
func extractFromTarXz(tarPath, extractDir string) (string, error) {
	// Use the tar command to extract the archive
	// This requires the tar command to be available on the system
	cmd := exec.Command("tar", "-xf", tarPath, "-C", extractDir)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract tar.xz: %v", err)
	}

	// Find the ffmpeg binary in the extracted files
	var ffmpegPath string
	err := filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (info.Name() == "ffmpeg" || info.Name() == "ffmpeg.exe") {
			ffmpegPath = path
			return io.EOF // Stop walking
		}

		return nil
	})

	if err != nil && err != io.EOF {
		return "", err
	}

	if ffmpegPath == "" {
		return "", fmt.Errorf("ffmpeg binary not found in extracted files")
	}

	// Move the binary to the root of the extract directory
	destPath := filepath.Join(extractDir, filepath.Base(ffmpegPath))
	if ffmpegPath != destPath {
		if err := os.Rename(ffmpegPath, destPath); err != nil {
			return "", fmt.Errorf("failed to move binary: %v", err)
		}
	}

	return destPath, nil
}
