#!/usr/bin/env bash
# release-docker.sh — Build and push Docker image to DockerHub.
#
# Usage: ./hack/release-docker.sh <tag> [dry_run]
#   tag:     Git tag (e.g., v0.2.0, v0.2.0-rc.1)
#   dry_run: "true" to skip push (default: false)
#
# Environment variables (required for push):
#   DOCKERHUB_USERNAME, DOCKERHUB_TOKEN

set -euo pipefail

TAG="${1:?Usage: $0 <tag> [dry_run]}"
DRY_RUN="${2:-false}"
# Treat empty string as non-dry-run (tag push trigger has no inputs)
[ -z "${DRY_RUN}" ] && DRY_RUN="false"
IMAGE="rebellions/rbln-container-toolkit"
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

# Login to DockerHub (skip for dry run)
if [ "${DRY_RUN}" != "true" ] && [ -n "${DOCKERHUB_USERNAME:-}" ] && [ -n "${DOCKERHUB_TOKEN:-}" ]; then
    echo "${DOCKERHUB_TOKEN}" | docker login -u "${DOCKERHUB_USERNAME}" --password-stdin
fi

# Determine push flag
PUSH_FLAG="--load"
if [ "${DRY_RUN}" != "true" ] && [ -n "${DOCKERHUB_USERNAME:-}" ]; then
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
