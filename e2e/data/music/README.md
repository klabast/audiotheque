# Audiotheque Test Audio Collection

This directory contains synthetic audio files generated specifically for testing Audiotheque's music library scanning, metadata
extraction, and playback functionality.

## Generation

All files are generated using FFmpeg and the `generate-test-audio.sh` script. The audio content is simple sine waves at
440Hz (A note) with varying frequencies for album tracks.

To regenerate the collection (run from the repo root):

```bash
cd scripts
docker build -f Dockerfile.test-audio -t test-audio-generator .
docker run --rm -v "$(git rev-parse --show-toplevel)/e2e/data/music:/output" test-audio-generator
```

## Test Coverage

### Hi-Res Audio (02-hi-res/)

Tests high-resolution audio detection (`BitDepth >= 24 && SampleRate >= 48000`):

- **24bit-96khz.flac** - 24-bit/96kHz FLAC (typical hi-res)
- **24bit-48khz.flac** - 24-bit/48kHz FLAC (minimum threshold edge case)

### Standard Audio (01-standard/various-formats/)

Tests various audio formats and bitrates:

- **16bit-44khz.flac** - 16-bit/44.1kHz FLAC (CD quality, NOT hi-res)
- **mp3-320kbps.mp3** - MP3 at 320kbps (high quality)
- **mp3-128kbps.mp3** - MP3 at 128kbps (standard quality)
- **aac-256kbps.m4a** - M4A/AAC at 256kbps
- **ogg-vorbis.ogg** - OGG Vorbis at quality 6
- **wav-uncompressed.wav** - Uncompressed WAV

### Edge Cases (01-standard/edge-cases/)

Tests edge case handling:

- **no-metadata.mp3** - File with all metadata stripped (empty tags)
- **unicode-名前-test.mp3** - Unicode filename and metadata (Japanese characters)
- **long-duration-10min.mp3** - 10-minute track (tests duration parsing)

### Complete Album (01-standard/album-complete/)

Tests album detection and track numbering:

- **01-track.flac** through **10-track.flac** - 10 tracks with proper metadata:
    - Album: "Complete Test Album"
    - Album Artist: "Audiotheque Test Band"
    - Track numbers: 1/10 through 10/10
    - Genre: "Test Rock"
    - Date: 2025

## File Properties Summary

| File                    | Format | Sample Rate | Bit Depth | Duration | Size   |
|-------------------------|--------|-------------|-----------|----------|--------|
| 24bit-96khz.flac        | FLAC   | 96kHz       | 24-bit    | 30s      | ~2.6MB |
| 24bit-48khz.flac        | FLAC   | 48kHz       | 24-bit    | 30s      | ~1.3MB |
| 16bit-44khz.flac        | FLAC   | 44.1kHz     | 16-bit    | 30s      | ~0.9MB |
| mp3-320kbps.mp3         | MP3    | 44.1kHz     | -         | 30s      | ~1.2MB |
| mp3-128kbps.mp3         | MP3    | 44.1kHz     | -         | 30s      | ~0.5MB |
| aac-256kbps.m4a         | AAC    | 44.1kHz     | -         | 30s      | ~1.0MB |
| ogg-vorbis.ogg          | OGG    | 48kHz       | -         | 30s      | ~0.9MB |
| wav-uncompressed.wav    | WAV    | 44.1kHz     | 16-bit    | 30s      | ~5.2MB |
| long-duration-10min.mp3 | MP3    | 44.1kHz     | -         | 600s     | ~23MB  |

**Total collection size:** ~31MB

## License

All files in this collection are synthetically generated audio (sine waves) with no copyright restrictions. They are
created specifically for testing purposes and contain no copyrighted musical content.

## Usage in Tests

These files are used by:

1. **E2E tests** (`features/library/*.feature`) - Library scanning, browsing, playback
2. **Unit tests** - Metadata extraction, format detection
3. **Manual testing** - Development and debugging

The small size (~31MB) makes this collection suitable for:

- Version control (checked into git)
- CI/CD pipelines
- Fast test execution
- Reproducible test environments
