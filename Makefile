# Audiotheque pipeline — runs locally and in CI.
#
# Stages mirror Dave Farley's deployment pipeline:
#   ci-commit     fast feedback (<5 min target): lint + check + unit + build
#   ci-acceptance long feedback: starts the stack and runs E2E
#
# Both stages also exist as their constituent targets so you can run
# any individual step locally.

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# Container engine: prefer docker (CI), fall back to podman (common on macOS).
# Override with `make DOCKER=docker ...` if needed.
DOCKER ?= $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null)

# Image references. Override IMAGE_REPO when building locally.
IMAGE_REPO ?= ghcr.io/klabast
TEST_MPD_IMAGE ?= $(IMAGE_REPO)/audiotheque-test-mpd:latest

GIT_SHA := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo dev)

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ----- ci-commit: fast feedback stage --------------------------------

.PHONY: ci-commit
ci-commit: lint check unit build ## Fast commit-stage pipeline (<5min target)

.PHONY: lint
lint: lint-server lint-ui ## Lint all code

.PHONY: lint-server
lint-server: ## go vet
	cd server && go vet ./...

.PHONY: lint-ui
lint-ui: ## prettier + eslint
	cd ui && npm run lint

.PHONY: check
check: check-ui ## Type / static checks (Go compile is covered by `unit`)

.PHONY: check-ui
check-ui: generate-api ## svelte-check
	cd ui && npm run check

.PHONY: unit
unit: unit-server unit-ui ## Run unit tests

.PHONY: unit-server
unit-server: ## go test (compiles all packages as a side effect)
	cd server && go test ./...

.PHONY: unit-ui
unit-ui: generate-api ## vitest
	cd ui && npm test

.PHONY: build
build: build-server build-ui ## Produce all artifacts

.PHONY: build-server
build-server: ## Build audiod binary with version metadata
	cd server && go build \
		-ldflags "-X 'audiod/internal/version.GitCommit=$(GIT_SHA)'" \
		-o audiod cmd/server/main.go

.PHONY: build-ui
build-ui: generate-api ## Build SvelteKit static bundle
	cd ui && npm run build

.PHONY: generate-api
generate-api: ## Regenerate OpenAPI client (requires swag + Java)
	cd ui && npm run generate-api

# ----- ci-acceptance: E2E stage --------------------------------------

.PHONY: ci-acceptance
ci-acceptance: e2e ## Acceptance pipeline: start stack, run E2E, tear down

.PHONY: e2e
e2e: e2e-up e2e-run e2e-down ## Run the full E2E suite (auto manages stack)

.PHONY: e2e-up
e2e-up: ## Start backend + test-mpd via docker-compose.test.yml
	$(DOCKER) compose -f docker-compose.test.yml up -d
	@echo "Waiting for stack to be ready..."
	@for i in $$(seq 1 60); do \
		if curl -sf http://localhost:8880/api/system >/dev/null 2>&1 \
			&& curl -sf http://localhost:6601/state >/dev/null 2>&1; then \
			echo "Stack ready"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	echo "Stack failed to come up"; \
	$(DOCKER) compose -f docker-compose.test.yml logs; \
	exit 1

.PHONY: e2e-run
e2e-run: ## Execute the cucumber suite against running stack (CI mode)
	cd e2e && CI_MODE=true DOCKER=$(DOCKER) npm test

.PHONY: e2e-down
e2e-down: ## Stop and remove the test stack
	$(DOCKER) compose -f docker-compose.test.yml down -v

# ----- test-mpd image ------------------------------------------------

.PHONY: test-mpd-image
test-mpd-image: ## Build the test-mpd image locally (matches CI publish)
	$(DOCKER) build -t $(TEST_MPD_IMAGE) tools/test-mpd

.PHONY: test-mpd-pull
test-mpd-pull: ## Pull the latest test-mpd image from GHCR
	$(DOCKER) pull $(TEST_MPD_IMAGE)

# ----- meta ----------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts
	rm -f server/audiod
	rm -rf ui/build ui/.svelte-kit

.PHONY: install
install: ## Install dev deps (UI npm + swag)
	cd ui && npm ci
	go install github.com/swaggo/swag/cmd/swag@latest
