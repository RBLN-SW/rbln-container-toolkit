#!/usr/bin/env bash
# release-build.sh — Build all binaries and run tests.
#
# Usage: ./hack/release-build.sh <tag>
#   tag: Git tag (e.g., v0.2.0, v0.2.0-rc.1)

set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"

echo "==> Building and testing for ${TAG}..."

make build VERSION="${TAG}"
make test
