# Audiotheque Server Makefile

# Variables
GOBIN ?= $(shell go env GOPATH)/bin
SWAG := $(GOBIN)/swag

# Version information
GIT_COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT_SHORT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev-$(GIT_COMMIT_SHORT)")
BUILD_TIME=$(shell date -u +"%Y-%m-%d %H:%M:%S UTC")

# Build flags with version info
LDFLAGS=-ldflags "\
	-X 'audiod/internal/version.Version=$(VERSION)' \
	-X 'audiod/internal/version.GitCommit=$(GIT_COMMIT)' \
	-X 'audiod/internal/version.BuildTime=$(BUILD_TIME)'"

# Default target
.PHONY: all
all: build

# Build the server
.PHONY: build
build:
	@echo "Building Audiotheque Server..."
	@echo "Version: $(VERSION)"
	@echo "Commit: $(GIT_COMMIT_SHORT)"
	@echo "Build Time: $(BUILD_TIME)"
	go build $(LDFLAGS) -o audiod cmd/server/main.go
	@echo "Build complete!"

# Run the server
.PHONY: run
run:
	go run cmd/server/main.go

# Run tests
.PHONY: test
test:
	go test ./...

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go install github.com/swaggo/swag/cmd/swag@latest

# Generate swagger documentation
.PHONY: docs
docs: deps
	@echo "Generating OpenAPI documentation..."
	$(SWAG) init -g cmd/server/main.go \
		--dir .,./internal/library,./internal/transcode,./internal/playback,./internal/mpd \
		--parseDependency \
		--parseInternal \
		--parseDepth 2

# Clean generated files
.PHONY: clean
clean:
	rm -f audiod
	rm -rf docs/

# Development mode with live reload
.PHONY: dev
dev: docs
	go run cmd/server/main.go

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make build  - Build the server binary"
	@echo "  make run    - Run the server"
	@echo "  make test   - Run tests"
	@echo "  make deps   - Install dependencies"
	@echo "  make docs   - Generate OpenAPI documentation"
	@echo "  make clean  - Clean generated files"
	@echo "  make dev    - Run in development mode"
	@echo "  make fmt    - Format code"
	@echo "  make lint   - Run linter"
	@echo "  make help   - Show this help message"

# Show version information
.PHONY: version
version:
	@echo "Version Information:"
	@echo "  Version:     $(VERSION)"
	@echo "  Git Commit:  $(GIT_COMMIT)"
	@echo "  Short Hash:  $(GIT_COMMIT_SHORT)"
	@echo "  Build Time:  $(BUILD_TIME)"