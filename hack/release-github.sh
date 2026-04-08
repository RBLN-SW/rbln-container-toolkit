#!/usr/bin/env bash
# release-github.sh — Create a GitHub Release on the public repository.
#
# Usage: ./hack/release-github.sh <tag>
#   tag: Git tag (e.g., v0.2.0, v0.2.0-rc.1)
#
# Environment variables:
#   GH_TOKEN — GitHub token with repo access

set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"
PUBLIC_REPO="RBLN-SW/rbln-container-toolkit"
CHANGELOG="CHANGELOG.md"

echo "==> Creating GitHub Release for ${TAG}..."

# Extract release notes from CHANGELOG.md
VERSION="${TAG#v}"
VERSION_BASE="${VERSION%%-rc*}"  # Strip RC suffix for changelog lookup
NOTES=""

if [ -f "${CHANGELOG}" ]; then
    NOTES=$(awk -v ver="${VERSION_BASE}" '
        /^## \[/ {
            if (found) exit
            if (index($0, "[v" ver "]") || index($0, "[" ver "]")) found=1
            next
        }
        found { print }
    ' "${CHANGELOG}")
fi

if [ -z "${NOTES}" ]; then
    NOTES="Release ${TAG}"
fi

NOTES_FILE=$(mktemp /tmp/release_notes.XXXXXX.md)
trap 'rm -f "${NOTES_FILE}"' EXIT
echo "${NOTES}" > "${NOTES_FILE}"

# Create release
if [[ "${TAG}" == *-rc* ]]; then
    echo "  Creating pre-release on ${PUBLIC_REPO}..."
    gh release create "${TAG}" \
        --repo "${PUBLIC_REPO}" \
        --title "${TAG}" \
        --notes-file "${NOTES_FILE}" \
        --prerelease
else
    echo "  Creating release on ${PUBLIC_REPO}..."
    gh release create "${TAG}" \
        --repo "${PUBLIC_REPO}" \
        --title "${TAG}" \
        --notes-file "${NOTES_FILE}"
fi

echo "==> GitHub Release created"
