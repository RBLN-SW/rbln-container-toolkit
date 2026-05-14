#!/usr/bin/env bash
# release-nexus-public.sh — Build DEB/RPM packages and publish them to the
# public Nexus (nexus.rebellions.ai). Mirrors the internal package job in
# release.yml but targets the externally-reachable apt-public / yum-public
# hosted repositories.
#
# Why rebuild instead of mirror from internal Nexus or download the release.yml
# artifact: rebuilding from the tag is reproducible, needs no extra secrets,
# and avoids the artifact retention cliff (release.yml keeps `packages` for
# only 3 days, while publish.yml can be dispatched weeks later).
#
# Usage: ./hack/release-nexus-public.sh <tag>
#   tag: Git tag (e.g., v0.2.0). RC tags are skipped automatically.
#
# Environment variables (all required for upload):
#   NEXUS_PUBLIC_URL, NEXUS_PUBLIC_USERNAME, NEXUS_PUBLIC_PASSWORD

set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"
VERSION="${TAG#v}"

# RC tags are intentionally not published to the public Nexus — once a
# package is in apt-public / yum-public, anonymous downloaders will pick it
# up and there is no clean way to recall it.
if [[ "${TAG}" == *-rc* ]]; then
    echo "==> RC tag detected (${TAG}), skipping public Nexus push"
    exit 0
fi

: "${NEXUS_PUBLIC_URL:?NEXUS_PUBLIC_URL is not set}"
: "${NEXUS_PUBLIC_USERNAME:?NEXUS_PUBLIC_USERNAME is not set}"
: "${NEXUS_PUBLIC_PASSWORD:?NEXUS_PUBLIC_PASSWORD is not set}"

# Sanitize version (matches release-package.sh)
VERSION="${VERSION//\//-}"

# Install nfpm if not present — keep this pinned in lock-step with
# release-package.sh so both flows ship byte-comparable packages even though
# they build independently.
NFPM_VERSION="v2.41.1"
which nfpm > /dev/null 2>&1 || go install "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VERSION}"

# Rebuild binaries and packages from the tag. Use `build-rblnml-ci` (cgo +
# with_rblnml tag, linked against the in-tree stub librbln-ml.so) so the
# binaries inside the package carry NEEDED librbln-ml.so — matching the
# nfpm-declared runtime depends and the CHANGELOG's documented intent for
# DEB/RPM. At runtime the dynamic linker resolves to the real librbln-ml.so
# pulled in by the package's depends from the user's UMD install.
rm -rf dist && mkdir -p dist
make build-rblnml-ci VERSION="${TAG}"
VERSION="${VERSION}" GOARCH=amd64 nfpm package --packager deb --target "dist/"
VERSION="${VERSION}" GOARCH=amd64 nfpm package --packager rpm --target "dist/"

echo "==> Packages built:"
ls -la dist/

NEXUS_BASE="${NEXUS_PUBLIC_URL%/}"

# Route credentials through a temporary curl config so they never appear on
# the command line. `-u user:pass` would expose them in /proc/<pid>/cmdline
# and `ps` output for any concurrent job sharing this self-hosted runner.
CURL_CFG="$(mktemp)"
trap 'rm -f "${CURL_CFG}"' EXIT
printf 'user = "%s:%s"\n' "${NEXUS_PUBLIC_USERNAME}" "${NEXUS_PUBLIC_PASSWORD}" > "${CURL_CFG}"

echo "==> Uploading to public Nexus (${NEXUS_BASE})..."

for pkg in dist/*.deb; do
    [ -f "${pkg}" ] || continue
    echo "  Uploading $(basename "${pkg}") to apt-public..."
    curl --fail --silent --show-error \
        --config "${CURL_CFG}" \
        -F "apt.asset=@${pkg}" \
        "${NEXUS_BASE}/service/rest/v1/components?repository=apt-public"
done

# Note on URL shape vs release-package.sh: the internal yum repo uploads to
# `/yum-internal/<distribution>/<basename>` (testing vs stable) because RC
# and final builds share one repo. Public Nexus is stable-only (RC was
# skipped above), so the layout collapses to a flat `/yum-public/<basename>`.
for pkg in dist/*.rpm; do
    [ -f "${pkg}" ] || continue
    echo "  Uploading $(basename "${pkg}") to yum-public..."
    curl --fail --silent --show-error \
        --config "${CURL_CFG}" \
        --upload-file "${pkg}" \
        "${NEXUS_BASE}/repository/yum-public/$(basename "${pkg}")"
done

echo "==> Public Nexus push complete"
