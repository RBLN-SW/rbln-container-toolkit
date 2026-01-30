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

.PHONY: all build build-ctk build-hook build-daemon clean test lint fmt vet vendor install help package package-deb package-rpm docker-build docker-push docker-builder ginkgo test-integration test-e2e test-e2e-local test-e2e-ubuntu2204 test-e2e-ubuntu2404 test-e2e-rhel9 test-e2e-all-images test-all clean-e2e generate

## all: Build everything
all: fmt vet lint test build

## build: Build all binaries (rbln-ctk, rbln-cdi-hook, and rbln-ctk-daemon)
build: build-ctk build-hook build-daemon

## build-ctk: Build the main CLI binary
build-ctk:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)

## build-hook: Build the CDI hook binary
build-hook:
	@echo "Building $(HOOK_BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) -ldflags "-X main.Version=$(VERSION)" -o $(BIN_DIR)/$(HOOK_BINARY_NAME) $(HOOK_CMD_DIR)

## build-daemon: Build the daemon binary
build-daemon:
	@echo "Building $(DAEMON_BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(DAEMON_BINARY_NAME) $(DAEMON_CMD_DIR)

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR) $(PKG_DIR)
	@$(GO) clean -cache -testcache

## test: Run unit tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
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

## package-deb: Build Debian package
package-deb: build
	@echo "Building Debian package..."
	@mkdir -p $(PKG_DIR)
	@which nfpm > /dev/null 2>&1 || (echo "Installing nfpm..." && go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest)
	VERSION=$(VERSION) GOARCH=$(GOARCH) nfpm package --packager deb --target $(PKG_DIR)/

## package-rpm: Build RPM package
package-rpm: build
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
