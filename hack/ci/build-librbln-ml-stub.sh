#!/usr/bin/env bash
# build-librbln-ml-stub.sh — compile the stub librbln-ml.so used by CI.
#
# Usage:
#   ./hack/ci/build-librbln-ml-stub.sh [OUT_DIR]
#
# Produces:
#   OUT_DIR/librbln-ml.so.1     (the stub shared library, soname=librbln-ml.so.1)
#   OUT_DIR/librbln-ml.so       (symlink so `ld -lrbln-ml` finds it)
#
# Pass the resulting directory to the linker via LIBRARY_PATH (build time)
# and LD_LIBRARY_PATH (runtime). See hack/ci/librbln-ml-stub/README.md for
# the maintenance contract — including when to add more symbols.

set -euo pipefail

# Linux-only — the script relies on GNU ld's `-soname` flag to set the
# resulting library's soname. macOS ld uses `-install_name` instead and
# would silently produce a binary the toolkit can't consume. Fail fast
# with a clear message so contributors on Mac don't chase the wrong tail.
if [[ "$(uname -s)" != "Linux" ]]; then
	echo "::error::$(basename "$0") only runs on Linux (relies on GNU ld -soname)." >&2
	echo "          The stub library is consumed by Linux CI runners — build the" >&2
	echo "          rblnml flavor on a Linux host or trust CI to exercise it." >&2
	exit 2
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STUB_SRC="${REPO_ROOT}/hack/ci/librbln-ml-stub/stub.c"
INCLUDE_DIR="${REPO_ROOT}/third_party/go-rbln-ml/include"
OUT_DIR="${1:-${REPO_ROOT}/build/ci-stub}"

if [[ ! -f "${STUB_SRC}" ]]; then
	echo "::error::stub source not found: ${STUB_SRC}" >&2
	exit 1
fi
if [[ ! -d "${INCLUDE_DIR}" ]]; then
	echo "::error::header directory missing: ${INCLUDE_DIR}" >&2
	echo "(third_party/go-rbln-ml/ is required so the stub matches the vendored ABI)" >&2
	exit 1
fi

mkdir -p "${OUT_DIR}"

# -fPIC: position-independent code, required for shared libs
# -Wl,-soname,librbln-ml.so.1: bakes the soname into the .so so the linker
#                              writes that exact NEEDED entry into consumers
# We don't need -Wall -Werror here: the stub is intentionally minimal and
# any cgo header change will surface as a real compile error anyway.
gcc -shared -fPIC \
	-I "${INCLUDE_DIR}" \
	-Wl,-soname,librbln-ml.so.1 \
	-o "${OUT_DIR}/librbln-ml.so.1" \
	"${STUB_SRC}"

ln -sf librbln-ml.so.1 "${OUT_DIR}/librbln-ml.so"

cat <<EOF
stub librbln-ml.so built: ${OUT_DIR}
to consume:
  export LIBRARY_PATH=${OUT_DIR}\${LIBRARY_PATH:+:\$LIBRARY_PATH}
  export LD_LIBRARY_PATH=${OUT_DIR}\${LD_LIBRARY_PATH:+:\$LD_LIBRARY_PATH}
  make build-rblnml
EOF
