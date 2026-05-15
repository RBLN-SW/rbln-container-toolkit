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

# upload_component POSTs an asset to the Nexus component API. A 4xx is only
# swallowed when the response body matches a known "asset already exists"
# signal — anything else (format error, auth failure, layout mismatch)
# stays a hard failure so a real first-time bug can't slip through as a
# "warning". The prior version swallowed every 4xx and let a depth-layout
# rejection masquerade as success, with the result that v0.2.0's RPM was
# silently missing from yum-public for hours.
#
# Known-good "already published" signals — extend this list when Nexus
# adds new wordings or when a different hosted repo type uses different
# phrasing:
#   * "Repository does not allow updating assets"   (current Nexus 3 default)
#   * "asset already exists"                         (older Nexus 3)
#   * "is already in use"                            (yum/apt component API)
ALREADY_PUBLISHED_REGEX='Repository does not allow updating assets|asset already exists|is already in use'

upload_component() {
    local desc="$1" repo="$2"
    shift 2
    local body status
    body="$(mktemp)"
    status="$(curl --silent --show-error \
        --config "${CURL_CFG}" \
        "$@" \
        -o "${body}" -w "%{http_code}" \
        "${NEXUS_BASE}/service/rest/v1/components?repository=${repo}")"
    case "${status}" in
        2*)
            echo "  ${desc}: OK (HTTP ${status})"
            ;;
        4*)
            if grep -qE "${ALREADY_PUBLISHED_REGEX}" "${body}"; then
                echo "  ${desc}: already published (HTTP ${status}) — skipping idempotently"
            else
                echo "  ${desc}: ERROR — HTTP ${status} (unrecognized 4xx, response body follows)" >&2
                cat "${body}" >&2
                rm -f "${body}"
                return 1
            fi
            ;;
        *)
            echo "  ${desc}: ERROR — HTTP ${status}" >&2
            cat "${body}" >&2
            rm -f "${body}"
            return 1
            ;;
    esac
    rm -f "${body}"
}

for pkg in dist/*.deb; do
    [ -f "${pkg}" ] || continue
    upload_component "$(basename "${pkg}") → apt-public" "apt-public" \
        -F "apt.asset=@${pkg}"
done

# yum-public is a Nexus hosted yum repo configured with `repodata-depth=3`,
# which rejects uploads whose in-repo path has fewer than three segments.
# Adopt a `stable/<arch>/Packages/<basename>` layout — mirrors the channel
# split used by the internal yum repo (testing/stable) so consumers register
# `baseurl=<NEXUS_BASE>/repository/yum-public/stable/<arch>/Packages/`.
# The arch is read off the nfpm-produced filename suffix so a future arm64
# build will land under `stable/aarch64/Packages/` without script changes.
for pkg in dist/*.rpm; do
    [ -f "${pkg}" ] || continue
    pkg_base="$(basename "${pkg}")"
    pkg_arch="${pkg_base##*.}"            # rbln-...-1.x86_64.rpm → rpm
    pkg_arch="${pkg_base%.${pkg_arch}}"   # rbln-...-1.x86_64
    pkg_arch="${pkg_arch##*.}"            # x86_64
    upload_component "${pkg_base} → yum-public" "yum-public" \
        -F "yum.asset=@${pkg}" \
        -F "yum.asset.filename=stable/${pkg_arch}/Packages/${pkg_base}"
done

echo "==> Public Nexus push complete"
