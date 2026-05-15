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

# upload_component POSTs an asset to the Nexus component API and tolerates
# any 4xx as "already published" so a partial-failure retry of this job is
# idempotent. Without this, retrying after a half-finished publish (e.g.
# DEB landed, RPM 400'd) trips Nexus's redeploy block on the asset that
# already made it through, forcing an admin to either flip
# "Allow redeploy" on the hosted repo or delete the orphan by hand.
# 5xx still fails the script — those are server problems we shouldn't
# silently swallow. Format errors on a fresh release will surface as 4xx
# on the *first* attempt, with the response body printed here, so the
# operator still sees them; the assumption is that they'll read the log
# before re-dispatching.
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
            echo "  ${desc}: WARNING — HTTP ${status} (treating as already-published; response body follows)"
            cat "${body}"
            echo
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

# RPM uses the same component API path as DEB. The previous flat-path PUT
# (`/repository/yum-public/<basename>`) tripped Nexus's yum hosted-repo
# layout check with HTTP 400; the component API has Nexus derive the
# in-repo path from `yum.asset.filename` regardless of repodata-depth.
for pkg in dist/*.rpm; do
    [ -f "${pkg}" ] || continue
    upload_component "$(basename "${pkg}") → yum-public" "yum-public" \
        -F "yum.asset=@${pkg}" \
        -F "yum.asset.filename=$(basename "${pkg}")"
done

echo "==> Public Nexus push complete"
