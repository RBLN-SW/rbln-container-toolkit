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

# This image targets Kubernetes deployments where rbln-ctk-daemon runs as a
# DaemonSet and the device-plugin (not the toolkit) owns NPU/RSD discovery.
# `setup.resolveTopology` short-circuits to NoopResolver{} whenever
# Devices.Disabled=true — the path the daemon takes on containerd/CRI-O —
# so it never calls into librbln-ml at runtime. That makes the cgo flavor
# pure dead weight here, and we keep the image as a static-Go build so it
# stays small and free of host-library dependencies. Standalone Docker
# users who want auto NPU↔RSD attachment should install the DEB/RPM
# package instead, which is the cgo flavor and pulls librbln-ml in.

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files (and the vendored go-rbln-ml replace target referenced
# from go.mod) first for better layer caching. third_party/ ships the
# bindings in-tree until the upstream module goes public.
COPY go.mod go.sum ./
COPY third_party/ ./third_party/
RUN go mod download

# Copy source code
COPY . .

# Build all binaries as static (CGO_ENABLED=0, no build tag).
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

# Install runtime dependencies (minimal). No librbln-ml here — the binaries
# are statically linked Go (see builder stage rationale) and the daemon
# runs in K8s mode where librbln-ml is never called.
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
