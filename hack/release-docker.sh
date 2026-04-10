#!/usr/bin/env bash
# release-docker.sh — Build and push Docker image to Harbor registry.
#
# Usage: ./hack/release-docker.sh <tag> [dry_run]
#   tag:     Git tag (e.g., v0.2.0, v0.2.0-rc.1)
#   dry_run: "true" to skip push (default: false)
#
# Environment variables (required for push):
#   SSW_HARBOR_URL, SSW_HARBOR_USERNAME, SSW_HARBOR_PASSWORD

set -euo pipefail

TAG="${1:?Usage: $0 <tag> [dry_run]}"
DRY_RUN="${2:-false}"
# Treat empty string as non-dry-run (tag push trigger has no inputs)
[ -z "${DRY_RUN}" ] && DRY_RUN="false"
# Strip protocol prefix if present (e.g., https://harbor.example.com → harbor.example.com)
REGISTRY="${SSW_HARBOR_URL:?SSW_HARBOR_URL is not set}"
REGISTRY="${REGISTRY#https://}"
REGISTRY="${REGISTRY#http://}"
IMAGE="${REGISTRY}/rebellions/rbln-container-toolkit"
DOCKERFILE="deployments/container/Dockerfile"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Sanitize tag for Docker (replace / with -)
DOCKER_TAG="${TAG//\//-}"

echo "==> Building Docker image ${IMAGE}:${DOCKER_TAG} (dry_run=${DRY_RUN})..."

# Determine tags
TAGS="-t ${IMAGE}:${DOCKER_TAG}"
if [[ "${TAG}" != *-rc* ]] && [[ "${TAG}" == v* ]]; then
    TAGS="${TAGS} -t ${IMAGE}:latest"
fi

# Login to Harbor (skip for dry run)
if [ "${DRY_RUN}" != "true" ] && [ -n "${SSW_HARBOR_USERNAME:-}" ] && [ -n "${SSW_HARBOR_PASSWORD:-}" ]; then
    echo "${SSW_HARBOR_PASSWORD}" | docker login "${REGISTRY}" -u "${SSW_HARBOR_USERNAME}" --password-stdin
fi

# Determine push flag
PUSH_FLAG="--load"
if [ "${DRY_RUN}" != "true" ] && [ -n "${SSW_HARBOR_USERNAME:-}" ]; then
    PUSH_FLAG="--push --sbom=true --provenance=mode=max"
fi

# shellcheck disable=SC2086
docker buildx build \
    --file "${DOCKERFILE}" \
    --build-arg VERSION="${TAG}" \
    --build-arg GIT_COMMIT="${GIT_COMMIT}" \
    --build-arg BUILD_DATE="${BUILD_DATE}" \
    --platform linux/amd64 \
    ${PUSH_FLAG} \
    ${TAGS} \
    .

echo "==> Docker build complete"
