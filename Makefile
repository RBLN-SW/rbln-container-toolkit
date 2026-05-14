# RBLN Container Toolkit Makefile

# Variables
BINARY_NAME := rbln-ctk
HOOK_BINARY_NAME := rbln-cdi-hook
DAEMON_BINARY_NAME := rbln-ctk-daemon
CMD_DIR := ./cmd/rbln-ctk
HOOK_CMD_DIR := ./cmd/rbln-cdi-hook
DAEMON_CMD_DIR := ./cmd/rbln-ctk-daemon
BIN_DIR := ./bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)"

# Go settings
GO := go
GOFLAGS := -v
GOOS ?= linux
GOARCH ?= amd64

# CGO + build tag controls.
#
# Default (CGO_ENABLED=0, no BUILD_TAGS): pure-Go static binary, no host
# dependencies — what the open-source toolkit ships as today and what CI
# defaults to so that runners without the Rebellions UMD can still build.
#
# Production deployments that want automatic NPU↔RSD attachment in the CDI
# spec must build with `BUILD_TAGS=with_rblnml CGO_ENABLED=1` (the convenience
# targets `build-rblnml` / `test-rblnml` set both). That requires `librbln-ml`
# (shared library + headers) installed on the build host; the resulting
# binary also needs `librbln-ml.so` reachable via the dynamic linker at
# runtime. Hosts where users run `rbln-ctk cdi generate` already have it as
# part of the driver install, so the runtime requirement is usually a no-op.
CGO_ENABLED ?= 0
BUILD_TAGS ?=
TAG_FLAGS := $(if $(BUILD_TAGS),-tags $(BUILD_TAGS),)

# Package directories
PKG_DIR := ./dist

# E2E Test Configuration
E2E_GINKGO_BIN := $(CURDIR)/bin/ginkgo
E2E_GINKGO_ARGS ?=
E2E_GINKGO_FOCUS ?=
E2E_LOG_DIR ?= $(CURDIR)/e2e_logs
GINKGO_VERSION := v2.22.0

# Supported base images
E2E_IMAGE_UBUNTU_2204 ?= ubuntu:22.04
E2E_IMAGE_UBUNTU_2404 ?= ubuntu:24.04
E2E_IMAGE_RHEL9 ?= redhat/ubi9:latest
E2E_IMAGE ?= $(E2E_IMAGE_UBUNTU_2204)

.PHONY: all build build-ctk build-hook build-daemon build-rblnml build-rblnml-ci ci-librbln-ml-stub clean test test-rblnml lint fmt vet vendor install help package package-deb package-rpm docker-build docker-push docker-builder ginkgo test-integration test-e2e test-e2e-local test-e2e-ubuntu2204 test-e2e-ubuntu2404 test-e2e-rhel9 test-e2e-all-images test-all clean-e2e generate

## all: Build everything
all: fmt vet lint test build

## build: Build all binaries (rbln-ctk, rbln-cdi-hook, and rbln-ctk-daemon)
build: build-ctk build-hook build-daemon

## build-ctk: Build the main CLI binary
build-ctk:
	@echo "Building $(BINARY_NAME) (CGO_ENABLED=$(CGO_ENABLED) BUILD_TAGS='$(BUILD_TAGS)')..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) $(TAG_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)

## build-hook: Build the CDI hook binary
build-hook:
	@echo "Building $(HOOK_BINARY_NAME) (CGO_ENABLED=$(CGO_ENABLED) BUILD_TAGS='$(BUILD_TAGS)')..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) $(TAG_FLAGS) -ldflags "-X main.Version=$(VERSION)" -o $(BIN_DIR)/$(HOOK_BINARY_NAME) $(HOOK_CMD_DIR)

## build-daemon: Build the daemon binary
build-daemon:
	@echo "Building $(DAEMON_BINARY_NAME) (CGO_ENABLED=$(CGO_ENABLED) BUILD_TAGS='$(BUILD_TAGS)')..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) $(TAG_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(DAEMON_BINARY_NAME) $(DAEMON_CMD_DIR)

## build-rblnml: Build all binaries with the librbln-ml-backed RSD resolver.
## Requires librbln-ml (shared library + headers) on the build host.
build-rblnml:
	@$(MAKE) CGO_ENABLED=1 BUILD_TAGS=with_rblnml build

## ci-librbln-ml-stub: Compile the CI-only stub librbln-ml.so under
## build/ci-stub/. Wires the resulting directory into LIBRARY_PATH /
## LD_LIBRARY_PATH for the current `make` invocation so a follow-up
## `make build-rblnml` finds the stub without further env setup. Linux
## only; the stub mirrors the upstream ABI via the bundled header.
ci-librbln-ml-stub:
	@./hack/ci/build-librbln-ml-stub.sh $(CURDIR)/build/ci-stub

## build-rblnml-ci: Convenience for CI runners — build the stub then
## immediately compile the rblnml flavor against it. Mirrors what the
## CI workflow does so contributors can reproduce a runner failure
## locally on a Linux host.
build-rblnml-ci: ci-librbln-ml-stub
	LIBRARY_PATH=$(CURDIR)/build/ci-stub$${LIBRARY_PATH:+:$$LIBRARY_PATH} \
	LD_LIBRARY_PATH=$(CURDIR)/build/ci-stub$${LD_LIBRARY_PATH:+:$$LD_LIBRARY_PATH} \
	$(MAKE) build-rblnml

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR) $(PKG_DIR)
	@$(GO) clean -cache -testcache

## test: Run unit tests
## Race detection requires cgo, so the test target intentionally lets
## CGO_ENABLED default to "1" on the host (Go's default on Linux) rather
## than pinning CGO_ENABLED=0. The static-binary contract only applies to
## build artifacts; tests are not shipped.
test:
	@echo "Running tests (BUILD_TAGS='$(BUILD_TAGS)')..."
	$(GO) test $(TAG_FLAGS) -v -race -cover ./...

## test-rblnml: Run unit tests with the librbln-ml-backed resolver path.
## Compiles every package against the rblnml cgo bindings; requires librbln-ml
## installed on the host. Tests that hit the real driver are gated separately.
test-rblnml:
	@$(MAKE) BUILD_TAGS=with_rblnml test

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test $(TAG_FLAGS) -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

## vendor: Vendor dependencies
vendor:
	@echo "Vendoring dependencies..."
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

## generate: Run go generate to create mocks
generate:
	@echo "Generating mocks..."
	@which moq > /dev/null 2>&1 || (echo "Installing moq..." && go install github.com/matryer/moq@latest)
	PATH="$(PATH):$(shell go env GOPATH)/bin" $(GO) generate ./...

## install: Install binaries to /usr/local/bin
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo install -m 755 $(BIN_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installing $(HOOK_BINARY_NAME) to /usr/local/bin..."
	sudo install -m 755 $(BIN_DIR)/$(HOOK_BINARY_NAME) /usr/local/bin/$(HOOK_BINARY_NAME)

## uninstall: Remove binaries from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstalling $(HOOK_BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(HOOK_BINARY_NAME)

## package: Build all packages (deb and rpm)
package: package-deb package-rpm

## package-deb: Build Debian package (cgo flavor — needs librbln-ml on the
## build host; the resulting .deb declares `librbln-ml` as a dependency).
package-deb: build-rblnml
	@echo "Building Debian package..."
	@mkdir -p $(PKG_DIR)
	@which nfpm > /dev/null 2>&1 || (echo "Installing nfpm..." && go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest)
	VERSION=$(VERSION) GOARCH=$(GOARCH) nfpm package --packager deb --target $(PKG_DIR)/

## package-rpm: Build RPM package (cgo flavor — needs librbln-ml on the
## build host; the resulting .rpm declares `librbln-ml` as a dependency).
package-rpm: build-rblnml
	@echo "Building RPM package..."
	@mkdir -p $(PKG_DIR)
	@which nfpm > /dev/null 2>&1 || (echo "Installing nfpm..." && go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest)
	VERSION=$(VERSION) GOARCH=$(GOARCH) nfpm package --packager rpm --target $(PKG_DIR)/

# Docker image settings
DOCKER_IMAGE ?= rebellions/rbln-container-toolkit
DOCKER_TAG ?= $(VERSION)
DOCKER_FILE := deployments/container/Dockerfile
BUILDX_BUILDER ?= rbln-builder

## docker-build: Build Docker image locally (no attestations)
docker-build:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker buildx build \
		--file $(DOCKER_FILE) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--platform linux/amd64 \
		--load \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		.

## docker-push: Build and push Docker image with supply chain attestations (SBOM + Provenance)
docker-push:
	@echo "Building and pushing $(DOCKER_IMAGE):$(DOCKER_TAG) with attestations..."
	docker buildx build \
		--file $(DOCKER_FILE) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--platform linux/amd64 \
		--sbom=true \
		--provenance=mode=max \
		--push \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		.

## docker-builder: Create buildx builder for attestation support (one-time setup)
docker-builder:
	@echo "Creating buildx builder '$(BUILDX_BUILDER)'..."
	docker buildx create --name=$(BUILDX_BUILDER) --driver=docker-container --bootstrap --use || \
		(echo "Builder already exists, switching to it..." && docker buildx use $(BUILDX_BUILDER))

## ginkgo: Install Ginkgo test framework binary
.PHONY: ginkgo
ginkgo: $(E2E_GINKGO_BIN)
$(E2E_GINKGO_BIN):
	@echo "Installing Ginkgo $(GINKGO_VERSION)..."
	@mkdir -p $(BIN_DIR)
	GOBIN=$(CURDIR)/bin go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

## test-integration: Run integration tests with build tag
.PHONY: test-integration
test-integration: build
	@echo "Running integration tests..."
	$(GO) test -tags=integration -v -race ./tests/integration/...

## test-e2e: Run E2E tests (requires Docker)
.PHONY: test-e2e
test-e2e: build ginkgo
	@echo "Running E2E tests with $(E2E_IMAGE)..."
	@mkdir -p $(E2E_LOG_DIR)
	cd tests/e2e && \
	E2E_BINARY_DIR=$(CURDIR)/bin \
	E2E_BASE_IMAGE=$(E2E_IMAGE) \
	E2E_RUN_MODE=container \
	$(E2E_GINKGO_BIN) $(E2E_GINKGO_ARGS) -v \
		--json-report $(E2E_LOG_DIR)/ginkgo.json \
		--label-filter="no-hardware" \
		--focus="$(E2E_GINKGO_FOCUS)" \
		./...

## test-e2e-local: Run E2E tests locally (no container isolation)
.PHONY: test-e2e-local
test-e2e-local: build ginkgo
	@echo "Running E2E tests (local mode)..."
	cd tests/e2e && \
	E2E_BINARY_DIR=$(CURDIR)/bin \
	E2E_RUN_MODE=local \
	$(E2E_GINKGO_BIN) $(E2E_GINKGO_ARGS) -v \
		--label-filter="no-hardware" \
		--focus="$(E2E_GINKGO_FOCUS)" \
		./...

## test-e2e-ubuntu2204: Run E2E tests on Ubuntu 22.04
.PHONY: test-e2e-ubuntu2204
test-e2e-ubuntu2204: E2E_IMAGE=$(E2E_IMAGE_UBUNTU_2204)
test-e2e-ubuntu2204: test-e2e

## test-e2e-ubuntu2404: Run E2E tests on Ubuntu 24.04
.PHONY: test-e2e-ubuntu2404
test-e2e-ubuntu2404: E2E_IMAGE=$(E2E_IMAGE_UBUNTU_2404)
test-e2e-ubuntu2404: test-e2e

## test-e2e-rhel9: Run E2E tests on RHEL 9 (UBI)
.PHONY: test-e2e-rhel9
test-e2e-rhel9: E2E_IMAGE=$(E2E_IMAGE_RHEL9)
test-e2e-rhel9: test-e2e

## test-e2e-all-images: Run E2E tests on all supported images
.PHONY: test-e2e-all-images
test-e2e-all-images: test-e2e-ubuntu2204 test-e2e-ubuntu2404 test-e2e-rhel9

## test-all: Run all tests (unit, integration, e2e)
.PHONY: test-all
test-all: test test-integration test-e2e

## clean-e2e: Clean E2E test artifacts
.PHONY: clean-e2e
clean-e2e:
	@echo "Cleaning E2E artifacts..."
	@rm -rf $(E2E_LOG_DIR)
	@docker rm -f rbln-e2e-test 2>/dev/null || true

## help: Show this help message
help:
	@echo "RBLN Container Toolkit"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

# Default target
.DEFAULT_GOAL := help
