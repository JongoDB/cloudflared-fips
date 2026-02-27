.PHONY: all build-all selftest-bin dashboard-bin tui-bin fips-proxy-bin \
       selftest setup status dashboard tui fips-proxy \
       dashboard-dev dashboard-build dashboard-embed \
       lint test test-cover vet check \
       manifest docker-build docs sbom crypto-audit clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
OUTPUT_DIR ?= build-output

LDFLAGS := -s -w -buildid= \
           -X github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo.Version=$(VERSION) \
           -X github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo.GitCommit=$(GIT_COMMIT) \
           -X github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo.BuildDate=$(BUILD_DATE) \
           -X github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo.FIPSBuild=true

# Platform-specific FIPS crypto backend selection
# BoringCrypto (CMVP #4735, FIPS 140-3): Linux only, requires CGO and a C compiler
#   matching the Go target architecture.
# Go native FIPS (CAVP A6650, CMVP pending): all platforms, no CGO required.
#
# We use BoringCrypto on Linux when the host machine architecture matches GOARCH
# (so the native C compiler can produce the right object code). Otherwise we fall
# back to Go native FIPS, which works everywhere without CGO.
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
GOARCH  := $(shell go env GOARCH 2>/dev/null)

# Map uname -m to Go arch names
ifeq ($(UNAME_M),x86_64)
  HOST_GOARCH := amd64
else ifeq ($(UNAME_M),aarch64)
  HOST_GOARCH := arm64
else
  HOST_GOARCH := $(UNAME_M)
endif

# Select FIPS backend
ifeq ($(UNAME_S),Linux)
  ifeq ($(HOST_GOARCH),$(GOARCH))
    # Native Linux — host arch matches Go target, CGO works
    GOEXPERIMENT ?= boringcrypto
    CGO_ENABLED ?= 1
    FIPS_ENV = GOEXPERIMENT=$(GOEXPERIMENT) CGO_ENABLED=$(CGO_ENABLED)
  else
    # Cross-arch Linux (e.g. arm64 host with amd64 Go) — CGO won't work
    CGO_ENABLED ?= 0
    GODEBUG ?= fips140=on
    FIPS_ENV = CGO_ENABLED=$(CGO_ENABLED) GODEBUG=$(GODEBUG)
  endif
else
  # macOS / Windows — BoringCrypto not available, use Go native FIPS
  CGO_ENABLED ?= 0
  GODEBUG ?= fips140=on
  FIPS_ENV = CGO_ENABLED=$(CGO_ENABLED) GODEBUG=$(GODEBUG)
endif

# ──────────────────────────────────────────────────────────────
# Build targets — produce all project binaries with FIPS crypto
# ──────────────────────────────────────────────────────────────

# Build all project binaries
all: build-all

build-all: selftest-bin dashboard-bin tui-bin fips-proxy-bin
	@echo "All binaries built in $(OUTPUT_DIR)/"
	@ls -lh $(OUTPUT_DIR)/

# Self-test CLI binary
selftest-bin:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/cloudflared-fips-selftest ./cmd/selftest

# Dashboard server binary (serves React frontend + compliance API)
dashboard-bin:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/cloudflared-fips-dashboard ./cmd/dashboard

# TUI binary (setup wizard + live status monitor)
tui-bin:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/cloudflared-fips-tui ./cmd/tui

# FIPS edge proxy binary (Tier 3 self-hosted)
fips-proxy-bin:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/cloudflared-fips-proxy ./cmd/fips-proxy

# ──────────────────────────────────────────────────────
# Run targets — build and execute in one step
# ──────────────────────────────────────────────────────

# Build and run the self-test suite
selftest: selftest-bin
	$(OUTPUT_DIR)/cloudflared-fips-selftest

# Run the setup wizard
setup:
	@$(FIPS_ENV) go run -ldflags "$(LDFLAGS)" ./cmd/tui setup

# Run the live compliance status monitor
status:
	@$(FIPS_ENV) go run -ldflags "$(LDFLAGS)" ./cmd/tui status

# Start the compliance dashboard API server (localhost:8080)
dashboard:
	@$(FIPS_ENV) go run -ldflags "$(LDFLAGS)" ./cmd/dashboard

# Build the TUI binary (alias for tui-bin)
tui: tui-bin

# Build the FIPS proxy binary (alias for fips-proxy-bin)
fips-proxy: fips-proxy-bin

# ──────────────────────────────────────────────────────
# Frontend targets
# ──────────────────────────────────────────────────────

# Start the React dashboard in development mode
dashboard-dev:
	cd dashboard && npm run dev

# Build the React dashboard for production
dashboard-build:
	cd dashboard && npm run build

# Build frontend, copy to embed dir, then rebuild dashboard binary
dashboard-embed: dashboard-build
	@rm -rf internal/dashboard/static
	@cp -r dashboard/dist internal/dashboard/static
	@echo "Copied dashboard/dist -> internal/dashboard/static for embedding"
	@$(MAKE) dashboard-bin

# ──────────────────────────────────────────────────────
# Quality targets
# ──────────────────────────────────────────────────────

# Run Go linters
lint:
	golangci-lint run ./...

# Run Go vet
vet:
	$(FIPS_ENV) go vet ./...

# Run all Go tests
test:
	$(FIPS_ENV) go test ./...

# Run tests with verbose output
test-verbose:
	$(FIPS_ENV) go test -v ./...

# Run tests with coverage report
test-cover:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go test -coverprofile=$(OUTPUT_DIR)/coverage.out ./...
	go tool cover -func=$(OUTPUT_DIR)/coverage.out
	@echo "HTML report: go tool cover -html=$(OUTPUT_DIR)/coverage.out"

# Run Puppeteer E2E tests for the dashboard
test-e2e:
	cd dashboard && npm run test:e2e

# Run all quality checks (vet + test)
check: vet test

# ──────────────────────────────────────────────────────
# Artifact generation
# ──────────────────────────────────────────────────────

# Generate build manifest
manifest:
	./scripts/generate-manifest.sh

# Build the FIPS Docker image
docker-build:
	docker build -f build/Dockerfile.fips \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t cloudflared-fips:$(VERSION) .

# Generate AO documentation package (PDF/HTML via pandoc)
docs:
	./scripts/generate-docs.sh docs/generated $(VERSION)

# Generate SBOMs (CycloneDX + SPDX)
sbom:
	./scripts/generate-sbom.sh upstream-cloudflared $(OUTPUT_DIR) $(VERSION)

# Run full crypto dependency audit
crypto-audit:
	./scripts/audit-crypto-deps.sh upstream-cloudflared $(OUTPUT_DIR)/crypto-audit-full.json

# ──────────────────────────────────────────────────────
# Clean
# ──────────────────────────────────────────────────────

# Clean build artifacts
clean:
	rm -rf $(OUTPUT_DIR)
	rm -rf dashboard/dist
