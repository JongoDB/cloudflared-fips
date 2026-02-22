.PHONY: build-fips selftest setup status tui dashboard-dev dashboard-build lint test manifest docker-build docs sbom crypto-audit clean

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
# Linux:         BoringCrypto via GOEXPERIMENT=boringcrypto (CMVP #4735, FIPS 140-3)
# macOS/Windows: Go native FIPS module via GODEBUG=fips140=on (CAVP A6650, CMVP pending)
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
  GOEXPERIMENT ?= boringcrypto
  CGO_ENABLED ?= 1
  FIPS_ENV = GOEXPERIMENT=$(GOEXPERIMENT) CGO_ENABLED=$(CGO_ENABLED)
else
  CGO_ENABLED ?= 0
  GODEBUG ?= fips140=on
  FIPS_ENV = CGO_ENABLED=$(CGO_ENABLED) GODEBUG=$(GODEBUG)
endif

# Build cloudflared with FIPS-validated cryptography
build-fips:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/cloudflared-fips ./cmd/selftest

# Build and run the standalone self-test binary
selftest:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/selftest ./cmd/selftest
	$(OUTPUT_DIR)/selftest

# Run the setup wizard
setup:
	@$(FIPS_ENV) go run -ldflags "$(LDFLAGS)" ./cmd/tui setup

# Run the live compliance status monitor
status:
	@$(FIPS_ENV) go run -ldflags "$(LDFLAGS)" ./cmd/tui status

# Build the TUI binary (optional — for distribution)
tui:
	@mkdir -p $(OUTPUT_DIR)
	$(FIPS_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/cloudflared-fips-tui ./cmd/tui

# Start the React dashboard in development mode
dashboard-dev:
	cd dashboard && npm run dev

# Build the React dashboard for production
dashboard-build:
	cd dashboard && npm run build

# Run Go linters
lint:
	golangci-lint run ./...

# Run Go tests
test:
	$(FIPS_ENV) go test -v ./...

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

# Clean build artifacts
clean:
	rm -rf $(OUTPUT_DIR)
	rm -rf dashboard/dist
	rm -rf dashboard/node_modules
