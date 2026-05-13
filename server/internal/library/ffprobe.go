package library

import (
	"encoding/json"
	"log"
	"os/exec"
	"strconv"
	"sync"

	"audiod/internal/ffmpeg"
)

// FFprobeResult represents the JSON output from ffprobe
type FFprobeResult struct {
	Streams []FFprobeStream `json:"streams"`
	Format  FFprobeFormat   `json:"format"`
}

// FFprobeStream represents an audio stream
type FFprobeStream struct {
	CodecType        string `json:"codec_type"`
	CodecName        string `json:"codec_name"`
	SampleRate       string `json:"sample_rate"`
	Channels         int    `json:"channels"`
	BitsPerRawSample string `json:"bits_per_raw_sample,omitempty"` // String in ffprobe output
	BitsPerSample    int    `json:"bits_per_sample,omitempty"`     // Sometimes an int
	SampleFmt        string `json:"sample_fmt"`
	Duration         string `json:"duration"`
	BitRate          string `json:"bit_rate"`
}

// FFprobeFormat represents the container format info
type FFprobeFormat struct {
	Duration string `json:"duration"`
	BitRate  string `json:"bit_rate"`
}

var (
	ffprobePath     string
	ffprobePathOnce sync.Once
)

// getFFprobePath returns the path to ffprobe, downloading if necessary (cached)
func getFFprobePath() string {
	ffprobePathOnce.Do(func() {
		ffprobePath = ffmpeg.EnsureFFprobe()
		if ffprobePath == "" {
			log.Println("Warning: ffprobe not available - audio properties won't be extracted")
		} else {
			log.Printf("Using ffprobe at: %s", ffprobePath)
		}
	})
	return ffprobePath
}

// RunFFprobe executes ffprobe and returns parsed results
func RunFFprobe(filePath string) (*FFprobeResult, error) {
	probePath := getFFprobePath()
	if probePath == "" {
		return nil, nil // ffprobe not available, return nil (not an error)
	}

	cmd := exec.Command(probePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "a:0", // First audio stream only
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result FFprobeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// extractAudioProperties uses FFprobe to fill audio properties in metadata
func extractAudioProperties(filePath string, meta *AudioMetadata) error {
	result, err := RunFFprobe(filePath)
	if err != nil {
		return err
	}
	if result == nil {
		return nil // ffprobe not available, skip audio properties
	}

	// Find audio stream
	var audioStream *FFprobeStream
	for i := range result.Streams {
		if result.Streams[i].CodecType == "audio" {
			audioStream = &result.Streams[i]
			break
		}
	}

	if audioStream == nil {
		return nil // No audio stream found, but not an error
	}

	// Codec name
	meta.Codec = audioStream.CodecName

	// Sample rate
	if rate, err := strconv.Atoi(audioStream.SampleRate); err == nil {
		meta.SampleRate = rate
	}

	// Channels
	meta.Channels = audioStream.Channels
	if meta.Channels == 0 {
		meta.Channels = 2 // Default to stereo
	}

	// Bit depth - try multiple sources
	meta.BitDepth = detectBitDepth(audioStream)

	// Duration (prefer stream duration, fallback to format)
	durationStr := audioStream.Duration
	if durationStr == "" {
		durationStr = result.Format.Duration
	}
	if dur, err := strconv.ParseFloat(durationStr, 64); err == nil {
		meta.Duration = int(dur * 1000) // Convert to milliseconds
	}

	// Bitrate (prefer stream, fallback to format)
	bitrateStr := audioStream.BitRate
	if bitrateStr == "" {
		bitrateStr = result.Format.BitRate
	}
	if br, err := strconv.Atoi(bitrateStr); err == nil {
		meta.Bitrate = br / 1000 // Convert to kbps
	}

	return nil
}

// detectBitDepth tries multiple methods to detect bit depth
func detectBitDepth(stream *FFprobeStream) int {
	// Method 1: bits_per_raw_sample (string in ffprobe JSON output)
	if stream.BitsPerRawSample != "" {
		if bits, err := strconv.Atoi(stream.BitsPerRawSample); err == nil && bits > 0 {
			return bits
		}
	}

	// Method 2: bits_per_sample (integer field)
	if stream.BitsPerSample > 0 {
		return stream.BitsPerSample
	}

	// Method 3: Sample format mapping
	switch stream.SampleFmt {
	case "u8", "u8p":
		return 8
	case "s16", "s16p":
		return 16
	case "s24", "s24p":
		return 24
	case "s32", "s32p", "flt", "fltp":
		return 32
	case "dbl", "dblp":
		return 64
	}

	// Default for lossy formats
	switch stream.CodecName {
	case "mp3", "aac", "ogg", "vorbis", "opus":
		return 0 // Lossy formats don't have meaningful bit depth
	case "flac", "alac":
		return 16 // Default assumption for lossless
	}

	return 0
}
