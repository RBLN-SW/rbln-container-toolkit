#!/usr/bin/env bash
# release-dockerhub.sh — Mirror a published image from Harbor to Docker Hub.
#
# Usage: ./hack/release-dockerhub.sh <tag>
#   tag: Git tag (e.g., v0.2.0). RC tags are skipped automatically.
#
# Environment variables (all required):
#   SSW_HARBOR_URL, SSW_HARBOR_USERNAME, SSW_HARBOR_PASSWORD
#   DOCKERHUB_USERNAME, DOCKERHUB_PASSWORD

set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"

# RC tags are not published to Docker Hub
if [[ "${TAG}" == *-rc* ]]; then
    echo "==> RC tag detected (${TAG}), skipping Docker Hub push"
    exit 0
fi

# Strip protocol prefix if present (e.g., https://harbor.example.com → harbor.example.com)
REGISTRY="${SSW_HARBOR_URL:?SSW_HARBOR_URL is not set}"
REGISTRY="${REGISTRY#https://}"
REGISTRY="${REGISTRY#http://}"

: "${SSW_HARBOR_USERNAME:?SSW_HARBOR_USERNAME is not set}"
: "${SSW_HARBOR_PASSWORD:?SSW_HARBOR_PASSWORD is not set}"
: "${DOCKERHUB_USERNAME:?DOCKERHUB_USERNAME is not set}"
: "${DOCKERHUB_PASSWORD:?DOCKERHUB_PASSWORD is not set}"

# Ensure docker login sessions are cleared on persistent self-hosted runners
cleanup() {
    docker logout "${REGISTRY}" >/dev/null 2>&1 || true
    docker logout docker.io >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Sanitize tag for Docker (replace / with -)
DOCKER_TAG="${TAG//\//-}"

REPO="rebellions/rbln-container-toolkit"
SRC="${REGISTRY}/${REPO}:${DOCKER_TAG}"
DST="docker.io/${REPO}"

echo "==> Mirroring ${SRC} → ${DST}:${DOCKER_TAG}"

echo "  Logging in to Harbor (${REGISTRY})..."
echo "${SSW_HARBOR_PASSWORD}" | docker login "${REGISTRY}" -u "${SSW_HARBOR_USERNAME}" --password-stdin

echo "  Logging in to Docker Hub..."
echo "${DOCKERHUB_PASSWORD}" | docker login docker.io -u "${DOCKERHUB_USERNAME}" --password-stdin

echo "  Pulling ${SRC}..."
docker pull "${SRC}"

echo "  Tagging and pushing ${DST}:${DOCKER_TAG}..."
docker tag "${SRC}" "${DST}:${DOCKER_TAG}"
docker push "${DST}:${DOCKER_TAG}"

# Stable v* tags also get :latest
if [[ "${TAG}" == v* ]]; then
    echo "  Tagging and pushing ${DST}:latest..."
    docker tag "${SRC}" "${DST}:latest"
    docker push "${DST}:latest"
fi

echo "==> Docker Hub push complete"
