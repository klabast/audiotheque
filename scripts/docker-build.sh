#!/bin/bash
# Docker build script with version information

# Get git information
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT_SHORT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev-${GIT_COMMIT_SHORT}")
BUILD_TIME=$(date -u +"%Y-%m-%d %H:%M:%S UTC")

# Docker image name
IMAGE_NAME="${IMAGE_NAME:-audiod}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

echo "Building Docker image: ${IMAGE_NAME}:${IMAGE_TAG}"
echo "Version: ${VERSION}"
echo "Commit: ${GIT_COMMIT_SHORT}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Build the Docker image with version arguments
docker build \
    --build-arg VERSION="${VERSION}" \
    --build-arg GIT_COMMIT="${GIT_COMMIT}" \
    --build-arg BUILD_TIME="${BUILD_TIME}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -t "${IMAGE_NAME}:${VERSION}" \
    -f Dockerfile \
    .

if [ $? -eq 0 ]; then
    echo ""
    echo "Docker build successful!"
    echo "Images created:"
    echo "  - ${IMAGE_NAME}:${IMAGE_TAG}"
    echo "  - ${IMAGE_NAME}:${VERSION}"
else
    echo "Docker build failed!"
    exit 1
fi