# Multi-stage Dockerfile for audiod.
#
# Stages:
#   1. ui-builder      builds the SvelteKit static bundle
#   2. server-builder  compiles the Go binary
#   3. runtime         tiny alpine + ffmpeg + the binary
#
# The OpenAPI spec (server/api/spec/) and TypeScript client
# (ui/src/lib/api/generated/) are produced on the host by
# `cd ui && npm run generate-api` (CI does this before `docker build`).
# Both are gitignored — the Dockerfile expects them in the build context.

ARG GO_VERSION=1.27
ARG ALPINE_VERSION=3.24
ARG NODE_VERSION=24

# ---------- 1. ui-builder ----------
FROM node:${NODE_VERSION} AS ui-builder

WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN npm ci

COPY ui/ ./
RUN npm run build

# ---------- 2. server-builder ----------
FROM golang:${GO_VERSION}-alpine AS server-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY server/go.mod server/go.sum* ./
RUN go mod download

COPY server/cmd ./cmd
COPY server/internal ./internal
COPY server/api/spec ./api/spec
COPY --from=ui-builder /app/build /app/web/

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# CGO disabled → fully static binary that runs on any alpine image.
# Migrations are go:embed-ed from internal/database/migrations, so no
# separate COPY is needed in either stage.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s \
    -X 'audiod/internal/version.Version=${VERSION}' \
    -X 'audiod/internal/version.GitCommit=${GIT_COMMIT}' \
    -X 'audiod/internal/version.BuildTime=${BUILD_TIME}'" \
    -o audiod ./cmd/server/main.go

# ---------- 3. runtime ----------
FROM alpine:${ALPINE_VERSION} AS runtime

RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    lame \
    opus-tools \
    flac

ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

WORKDIR /app
RUN mkdir -p /app/music /app/transcode-cache /app/data

COPY --from=server-builder /app/audiod /app/
COPY --from=server-builder /app/web /app/web

EXPOSE 8080

ENV MUSIC_LIBRARY_PATH=/app/music
ENV AUDIOD_DATA_DIR=/app/data

CMD ["/app/audiod"]
