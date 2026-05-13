#!/bin/bash
# Build script for Audiotheque server with version information

# Get git information
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT_SHORT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev-${GIT_COMMIT_SHORT}")
BUILD_TIME=$(date -u +"%Y-%m-%d %H:%M:%S UTC")

# Set build directory
BUILD_DIR="${BUILD_DIR:-./}"
OUTPUT_NAME="${OUTPUT_NAME:-audiod}"

echo "Building Audiotheque Server..."
echo "Version: ${VERSION}"
echo "Commit: ${GIT_COMMIT}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Build with version information injected via ldflags
go build -ldflags "\
    -X 'audiod/internal/version.Version=${VERSION}' \
    -X 'audiod/internal/version.GitCommit=${GIT_COMMIT}' \
    -X 'audiod/internal/version.BuildTime=${BUILD_TIME}'" \
    -o "${BUILD_DIR}/${OUTPUT_NAME}" \
    cmd/server/main.go

if [ $? -eq 0 ]; then
    echo "Build successful: ${BUILD_DIR}/${OUTPUT_NAME}"
    echo ""
    echo "Version info:"
    echo "  Version:   ${VERSION}"
    echo "  Commit:    ${GIT_COMMIT_SHORT}"
    echo "  Full Hash: ${GIT_COMMIT}"
    echo "  Built:     ${BUILD_TIME}"
else
    echo "Build failed!"
    exit 1
fi