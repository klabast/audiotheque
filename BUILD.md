# Building Audiotheque

## Docker Build Process

Audiotheque uses a multi-stage Docker build process with pre-built base images to speed up builds and ensure consistency.

### Pre-built Base Images

We maintain pre-built base images that include:
- Alpine Linux 3.21
- FFmpeg and audio codecs (lame, opus, flac)
- Go 1.24 (builder image)
- Node.js LTS (builder image)
- All required runtime dependencies

Base images are automatically rebuilt weekly to include security updates.

### Building Locally

#### Quick Build (uses pre-built base if available)
```bash
docker build -t audiotheque:latest .
```

#### Build with Local Base Images
```bash
# Build base images first (only needed once)
docker-compose -f docker-compose.build.yaml build audiod-base

# Then build the main application
docker-compose -f docker-compose.build.yaml build audiod
```

#### Build Everything from Scratch
```bash
# This will build without using pre-built bases
docker build \
  --build-arg BASE_IMAGE=golang:1.24-alpine \
  --build-arg RUNTIME_IMAGE=alpine:3.21 \
  -t audiotheque:latest .
```

### GitHub Actions

Our CI/CD pipeline:
1. Builds and caches base images weekly (or on manual trigger)
2. Runs tests with FFmpeg installed
3. Builds multi-architecture images (amd64, arm64)
4. Pushes images to GitHub Container Registry

### Build Arguments

- `BASE_IMAGE`: Builder base image (default: `ghcr.io/klabast/audiotheque-base:builder`)
- `RUNTIME_IMAGE`: Runtime base image (default: `ghcr.io/klabast/audiotheque-base:latest`)

### Multi-Architecture Builds

To build for multiple architectures locally:
```bash
# Enable buildx
docker buildx create --use

# Build for multiple platforms
docker buildx build --platform linux/amd64,linux/arm64 -t audiotheque:latest .
```

### Development Workflow

For development, you can mount your local code:
```bash
docker-compose up -d
```

This uses the regular docker-compose.yaml which builds from source.

### Troubleshooting

If base images are not available:
1. The Dockerfile will fall back to building from scratch
2. You can manually trigger base image build: Actions → "Build Base Docker Image" → Run workflow
3. Or build locally using docker-compose.build.yaml