#!/usr/bin/env bash
# release-build.sh — Build all binaries and run tests.
#
# Usage: ./hack/release-build.sh <tag>
#   tag: Git tag (e.g., v0.2.0, v0.2.0-rc.1)

set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"

echo "==> Building and testing for ${TAG}..."

# Build the rblnml flavor against the in-tree stub librbln-ml.so so the
# resulting binaries carry NEEDED librbln-ml.so. This matches the
# CHANGELOG-documented intent for DEB/RPM packages (nfpm declares
# `librbln-ml` as a runtime depends) and lets the librbln-ml-backed
# RsdResolver kick in on production hosts where the real UMD is installed.
# The stub is link-time only; the dynamic linker resolves to the real
# library at runtime via the package's depends.
make build-rblnml-ci VERSION="${TAG}"
make test
