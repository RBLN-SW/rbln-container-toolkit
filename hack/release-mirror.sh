#!/usr/bin/env bash
# release-mirror.sh — Push code and tag to the public repository.
#
# Usage: ./hack/release-mirror.sh <tag>
#   tag: Git tag (e.g., v0.2.0). RC tags are skipped automatically.
#
# Environment variables:
#   PUBLIC_REPO_DEPLOY_KEY — SSH deploy key for the public repository

set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"
PUBLIC_REPO="RBLN-SW/rbln-container-toolkit"

# RC tags should not be mirrored
if [[ "${TAG}" == *-rc* ]]; then
    echo "==> RC tag detected (${TAG}), skipping mirror push"
    exit 0
fi

echo "==> Mirroring ${TAG} to ${PUBLIC_REPO}..."

# Configure authentication
if [ -z "${PUBLIC_REPO_DEPLOY_KEY:-}" ]; then
    echo "::error::PUBLIC_REPO_DEPLOY_KEY is not set"
    exit 1
fi

echo "  Using deploy key..."
mkdir -p ~/.ssh
echo "${PUBLIC_REPO_DEPLOY_KEY}" > ~/.ssh/deploy_key
chmod 600 ~/.ssh/deploy_key
trap 'rm -f ~/.ssh/deploy_key' EXIT

# Pre-populate known_hosts with GitHub's published fingerprint
ssh-keyscan -H github.com >> ~/.ssh/known_hosts 2>/dev/null
export GIT_SSH_COMMAND="ssh -i ~/.ssh/deploy_key -o UserKnownHostsFile=${HOME}/.ssh/known_hosts"
REMOTE_URL="git@github.com:${PUBLIC_REPO}.git"

git remote add public "${REMOTE_URL}" 2>/dev/null || git remote set-url public "${REMOTE_URL}"

echo "  Pushing main branch..."
git push public HEAD:main --force-with-lease

echo "  Pushing tag ${TAG}..."
git push public "${TAG}"

echo "==> Mirror push complete"
