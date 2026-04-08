#!/usr/bin/env bash
# release-package.sh — Build DEB/RPM packages and upload to Nexus.
#
# Usage: ./hack/release-package.sh <tag> [dry_run]
#   tag:     Git tag (e.g., v0.2.0, v0.2.0-rc.1)
#   dry_run: "true" to skip Nexus upload (default: false)
#
# Environment variables (required for upload):
#   NEXUS_URL, NEXUS_USERNAME, NEXUS_PASSWORD

set -euo pipefail

TAG="${1:?Usage: $0 <tag> [dry_run]}"
DRY_RUN="${2:-false}"
# Treat empty string as non-dry-run (tag push trigger has no inputs)
[ -z "${DRY_RUN}" ] && DRY_RUN="false"
VERSION="${TAG#v}"  # Strip leading 'v' for package version

echo "==> Packaging ${TAG} (dry_run=${DRY_RUN})..."

# Sanitize version for packaging (strip leading 'v', replace / with -)
VERSION="${VERSION//\//-}"

# Install nfpm if not present (pinned version for reproducible builds)
NFPM_VERSION="v2.41.1"
which nfpm > /dev/null 2>&1 || go install "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VERSION}"

# Clean and create dist directory
rm -rf dist && mkdir -p dist

# Build packages
VERSION="${VERSION}" GOARCH=amd64 nfpm package --packager deb --target "dist/"
VERSION="${VERSION}" GOARCH=amd64 nfpm package --packager rpm --target "dist/"

echo "==> Packages built:"
ls -la dist/

# Upload to Nexus
if [ "${DRY_RUN}" = "true" ]; then
    echo "==> Dry run: skipping Nexus upload"
    exit 0
fi

if [ -z "${NEXUS_URL:-}" ] || [ -z "${NEXUS_USERNAME:-}" ] || [ -z "${NEXUS_PASSWORD:-}" ]; then
    echo "::warning::Nexus credentials not configured, skipping upload"
    exit 0
fi

# RC tags go to test repo, release tags go to prod repo
if [[ "${TAG}" == *-rc* ]]; then
    REPO_SUFFIX="test"
else
    REPO_SUFFIX="prod"
fi

# Extract hostname from NEXUS_URL for netrc
NEXUS_HOST="$(echo "${NEXUS_URL}" | sed 's|https\?://||' | sed 's|/.*||')"

echo "==> Uploading to Nexus (${REPO_SUFFIX})..."

for pkg in dist/*.deb; do
    [ -f "${pkg}" ] || continue
    echo "  Uploading $(basename "${pkg}")..."
    curl --fail \
        --netrc-file <(printf "machine %s\nlogin %s\npassword %s\n" \
            "${NEXUS_HOST}" "${NEXUS_USERNAME}" "${NEXUS_PASSWORD}") \
        --upload-file "${pkg}" \
        "${NEXUS_URL}/repository/rbln-ctk-deb-${REPO_SUFFIX}/$(basename "${pkg}")"
done

for pkg in dist/*.rpm; do
    [ -f "${pkg}" ] || continue
    echo "  Uploading $(basename "${pkg}")..."
    curl --fail \
        --netrc-file <(printf "machine %s\nlogin %s\npassword %s\n" \
            "${NEXUS_HOST}" "${NEXUS_USERNAME}" "${NEXUS_PASSWORD}") \
        --upload-file "${pkg}" \
        "${NEXUS_URL}/repository/rbln-ctk-rpm-${REPO_SUFFIX}/$(basename "${pkg}")"
done

echo "==> Nexus upload complete"
