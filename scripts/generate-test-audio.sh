#!/bin/bash
set -e

# Generate test audio files for Audiotheque testing
# Creates files with specific formats, bitrates, and metadata

OUTPUT_DIR="${OUTPUT_DIR:-/output}"

echo "🎵 Generating test audio files..."

# Check for ffmpeg
if ! command -v ffmpeg &> /dev/null; then
    echo "❌ ffmpeg is required but not installed"
    echo "Install with: brew install ffmpeg"
    exit 1
fi

# Create directory structure
mkdir -p "$OUTPUT_DIR"/{01-standard/{album-complete,various-formats,edge-cases},02-hi-res}

# Generate base audio (30 seconds of sine wave at 440Hz - A note)
echo "📢 Generating base audio (30 second sine wave)..."

# 1. Hi-Res 24-bit/96kHz FLAC
echo "  → 24-bit/96kHz FLAC (hi-res)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -ar 96000 -sample_fmt s32 \
    -metadata title="Hi-Res Test Track" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - Hi-Res" \
    -metadata date="2025" \
    -metadata genre="Test" \
    -y "$OUTPUT_DIR/02-hi-res/24bit-96khz.flac" 2>/dev/null

# 2. Hi-Res 24-bit/48kHz FLAC (edge case - minimum for hi-res)
echo "  → 24-bit/48kHz FLAC (hi-res edge case)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -ar 48000 -sample_fmt s32 \
    -metadata title="Hi-Res Edge Case" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - Hi-Res" \
    -metadata date="2025" \
    -y "$OUTPUT_DIR/02-hi-res/24bit-48khz.flac" 2>/dev/null

# 3. Standard 16-bit/44.1kHz FLAC
echo "  → 16-bit/44.1kHz FLAC (CD quality)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -ar 44100 -sample_fmt s16 \
    -metadata title="Standard Quality Track" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - Standard" \
    -metadata date="2025" \
    -y "$OUTPUT_DIR/01-standard/various-formats/16bit-44khz.flac" 2>/dev/null

# 4. MP3 320kbps
echo "  → MP3 320kbps"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -b:a 320k \
    -metadata title="High Quality MP3" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - MP3" \
    -metadata date="2025" \
    -y "$OUTPUT_DIR/01-standard/various-formats/mp3-320kbps.mp3" 2>/dev/null

# 5. MP3 128kbps
echo "  → MP3 128kbps"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -b:a 128k \
    -metadata title="Standard Quality MP3" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - MP3" \
    -y "$OUTPUT_DIR/01-standard/various-formats/mp3-128kbps.mp3" 2>/dev/null

# 6. M4A/AAC
echo "  → M4A/AAC"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -c:a aac -b:a 256k \
    -metadata title="AAC Test Track" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - AAC" \
    -y "$OUTPUT_DIR/01-standard/various-formats/aac-256kbps.m4a" 2>/dev/null

# 7. OGG Vorbis
echo "  → OGG Vorbis"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -c:a libvorbis -q:a 6 \
    -metadata title="OGG Vorbis Track" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - OGG" \
    -y "$OUTPUT_DIR/01-standard/various-formats/ogg-vorbis.ogg" 2>/dev/null

# 8. WAV (uncompressed)
echo "  → WAV (uncompressed)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -ar 44100 \
    -metadata title="WAV Test Track" \
    -metadata artist="Audiotheque Test Suite" \
    -y "$OUTPUT_DIR/01-standard/various-formats/wav-uncompressed.wav" 2>/dev/null

# 9. File with NO metadata (edge case)
echo "  → No metadata (edge case)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -b:a 192k \
    -map_metadata -1 \
    -y "$OUTPUT_DIR/01-standard/edge-cases/no-metadata.mp3" 2>/dev/null

# 10. Unicode filename (edge case)
echo "  → Unicode filename (edge case)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 2 -b:a 192k \
    -metadata title="Test Track 名前" \
    -metadata artist="Artist 藝術家" \
    -y "$OUTPUT_DIR/01-standard/edge-cases/unicode-名前-test.mp3" 2>/dev/null

# 11. Long duration (10 minutes)
echo "  → Long duration track (10 minutes)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=600" -ac 2 -b:a 192k \
    -metadata title="Long Duration Track" \
    -metadata artist="Audiotheque Test Suite" \
    -metadata album="Test Album - Long" \
    -y "$OUTPUT_DIR/01-standard/edge-cases/long-duration-10min.mp3" 2>/dev/null

# 12-21. Complete album (10 tracks)
echo "📀 Generating complete album (10 tracks)..."
for i in {1..10}; do
    track_num=$(printf "%02d" $i)
    echo "  → Track $track_num"
    ffmpeg -f lavfi -i "sine=frequency=$((440 + i * 10)):duration=30" -ac 2 -ar 44100 -sample_fmt s16 \
        -metadata title="Track $track_num - Album Test" \
        -metadata artist="Audiotheque Test Band" \
        -metadata album="Complete Test Album" \
        -metadata album_artist="Audiotheque Test Band" \
        -metadata date="2025" \
        -metadata track="$i/10" \
        -metadata genre="Test Rock" \
        -y "$OUTPUT_DIR/01-standard/album-complete/${track_num}-track.flac" 2>/dev/null
done

echo ""
echo "✅ Test audio generation complete!"
echo ""
echo "📊 Summary:"
du -sh "$OUTPUT_DIR"
echo ""
find "$OUTPUT_DIR" -type f | wc -l | xargs echo "Total files:"
echo ""
echo "📂 Structure:"
tree -L 3 "$OUTPUT_DIR" 2>/dev/null || find "$OUTPUT_DIR" -type f
