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

Our CI/CD pipeline (`.github/workflows/ci.yml`) is a Dave Farley-style
deployment pipeline. Each stage gates the next:

1. **Commit** — `go vet`, Go unit tests, and the binary; UI lint, type check,
   unit tests and build. Under five minutes, runs on every event.
2. **Image** — builds the release candidate once for `linux/amd64` and pushes
   it to GHCR tagged `sha-<short>`.
3. **Acceptance** — pulls that exact image, brings up the stack from
   `docker-compose.test.yml`, and runs the Cucumber/Playwright suite across
   desktop, tablet and mobile.
4. **Promote** — retags the candidate as `:latest`. Only on `main`, and only
   after acceptance passed.

The candidate is built once and promoted by retag, never rebuilt, so the image
carrying `:latest` is bit-identical to the one acceptance ran against.

Pull requests run stages 1–3, so nothing reaches `main` without having passed
E2E. Trunk pipelines are serialised by a `concurrency` group — promote retags a
shared tag, and two runs finishing out of order would leave `:latest` on the
older commit with every job green.

See `docs/adr/0001-trunk-based-delivery-pipeline.md`.

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