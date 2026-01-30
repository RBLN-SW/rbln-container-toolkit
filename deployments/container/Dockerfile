# RBLN Container Toolkit Docker Image
#
# This multi-stage Dockerfile builds all toolkit binaries and creates
# a container image for Kubernetes DaemonSet deployment.
#
# Binaries included:
#   - rbln-ctk:        CLI tool for CDI generation and runtime configuration
#   - rbln-ctk-daemon: Daemon for Kubernetes DaemonSet with health probes
#   - rbln-cdi-hook:   CDI hook for ldcache updates (copied to host)
#
# Build (local):
#   make docker-build
#
# Build & push with attestations (requires: make docker-builder):
#   make docker-push
#
# Or manually:
#   docker buildx build --sbom=true --provenance=mode=max \
#     --build-arg VERSION=v1.0.0 --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
#     -t rebellions/rbln-container-toolkit:v1.0.0 \
#     -f deployments/container/Dockerfile --push .
#
# Run as DaemonSet (default):
#   The image runs rbln-ctk-daemon by default for Kubernetes deployments.
#   See deployments/kubernetes/daemonset.yaml for full configuration.
#
# Run standalone (CDI generation only):
#   docker run --rm -v /var/run/cdi:/var/run/cdi \
#     rebellions/rbln-container-toolkit:latest rbln-ctk cdi generate
#

# Build stage
FROM golang:1.24-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w \
        -X main.version=${VERSION} \
        -X main.gitCommit=${GIT_COMMIT} \
        -X main.buildDate=${BUILD_DATE}" \
    -o rbln-ctk \
    ./cmd/rbln-ctk && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o rbln-ctk-daemon \
    ./cmd/rbln-ctk-daemon && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o rbln-cdi-hook \
    ./cmd/rbln-cdi-hook

# Final stage - minimal image
FROM alpine:3.19

# Labels
LABEL org.opencontainers.image.title="RBLN Container Toolkit"
LABEL org.opencontainers.image.description="Container toolkit for Rebellions NPU devices"
LABEL org.opencontainers.image.vendor="Rebellions"
LABEL org.opencontainers.image.source="https://github.com/RBLN-SW/rbln-container-toolkit"

# Install runtime dependencies (minimal)
RUN apk add --no-cache \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 rbln && \
    adduser -u 1000 -G rbln -s /bin/sh -D rbln

# Copy binaries from builder
COPY --from=builder /build/rbln-ctk /usr/bin/rbln-ctk
COPY --from=builder /build/rbln-ctk-daemon /usr/bin/rbln-ctk-daemon
COPY --from=builder /build/rbln-cdi-hook /usr/local/bin/rbln-cdi-hook

# Create necessary directories
RUN mkdir -p /var/run/cdi /etc/rbln && \
    chown -R rbln:rbln /var/run/cdi /etc/rbln

# Copy default configuration
COPY config/container-toolkit.yaml /etc/rbln/container-toolkit.yaml

# Set environment variables
ENV RBLN_CTK_CONFIG=/etc/rbln/container-toolkit.yaml
ENV RBLN_CDI_OUTPUT=/var/run/cdi/rbln.yaml

# Default working directory
WORKDIR /

# Run as root by default (needed for some host path access)
# Can be overridden in Kubernetes with securityContext
USER root

# Default: run as daemon for Kubernetes DaemonSet
ENTRYPOINT ["/usr/bin/rbln-ctk-daemon"]
CMD []
