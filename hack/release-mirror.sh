#!/usr/bin/env bash
# release-mirror.sh — Push code and tag to the public repository.
#
# Usage: ./hack/release-mirror.sh <tag>
#   tag: Git tag (e.g., v0.2.0). RC tags are skipped automatically.
#
# Environment variables (one required):
#   PUBLIC_REPO_DEPLOY_KEY — SSH deploy key (preferred)
#   PUBLIC_REPO_PAT        — Personal access token (fallback)

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
if [ -n "${PUBLIC_REPO_DEPLOY_KEY:-}" ]; then
    echo "  Using deploy key..."
    mkdir -p ~/.ssh
    echo "${PUBLIC_REPO_DEPLOY_KEY}" > ~/.ssh/deploy_key
    chmod 600 ~/.ssh/deploy_key
    trap 'rm -f ~/.ssh/deploy_key' EXIT

    # Pre-populate known_hosts with GitHub's published fingerprint
    ssh-keyscan -H github.com >> ~/.ssh/known_hosts 2>/dev/null
    export GIT_SSH_COMMAND="ssh -i ~/.ssh/deploy_key -o UserKnownHostsFile=${HOME}/.ssh/known_hosts"
    REMOTE_URL="git@github.com:${PUBLIC_REPO}.git"
elif [ -n "${PUBLIC_REPO_PAT:-}" ]; then
    echo "  Using PAT..."
    # Use credential helper to avoid embedding token in remote URL
    git config --local credential.helper \
        "!f() { printf 'username=x-access-token\npassword=%s\n' \"${PUBLIC_REPO_PAT}\"; }; f"
    REMOTE_URL="https://github.com/${PUBLIC_REPO}.git"
else
    echo "::error::Neither PUBLIC_REPO_DEPLOY_KEY nor PUBLIC_REPO_PAT is set"
    exit 1
fi

git remote add public "${REMOTE_URL}" 2>/dev/null || git remote set-url public "${REMOTE_URL}"

echo "  Pushing main branch..."
git push public HEAD:main --force-with-lease

echo "  Pushing tag ${TAG}..."
git push public "${TAG}"

echo "==> Mirror push complete"
