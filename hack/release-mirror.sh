#!/usr/bin/env bash
# release-mirror.sh — Push code and tag to the public repository.
#
# Usage: ./hack/release-mirror.sh <tag>
#   tag: Git tag (e.g., v0.2.0). RC tags are skipped automatically.
#
# Environment variables:
#   PUBLIC_REPO_DEPLOY_KEY — SSH deploy key for the public repository
#   MIRROR_DRY_RUN         — set to 1 to rewrite history locally and print what
#                            would be pushed without contacting the public
#                            repository (no deploy key needed)
#   INTERNAL_TRACKER_KEYS  — passed through to sanitize-commit-message.sh
#
# History rewrite
# ---------------
# Commit messages on the internal main carry references that mean nothing —
# or point at private resources — once mirrored: Linear/Jira ticket IDs, the
# `(#NN)` pull-request numbers GitHub appends on squash merge, and AI
# co-author trailers. Before pushing, this script clones the checkout into a
# scratch directory and rewrites every commit message through
# hack/sanitize-commit-message.sh (`git filter-branch --msg-filter`), carrying
# tags across to the rewritten commits. Author, committer, dates and trees are
# untouched, so the rewrite is deterministic: re-running on the same input
# yields the same public SHAs and each release only appends. Public commit
# SHAs therefore differ from the internal ones by design.
#
# Because every stable tag on the public side must point into the rewritten
# history, all non-RC tags — not just the one being released — are
# force-pushed on every run. Those pushes are no-ops once the SHAs match.
#
# `git filter-branch` is used instead of `git filter-repo` because it ships
# with git itself; the runner needs no extra package. Its deprecation warning
# is about shape-changing rewrites (path filters, index filters) that this
# message-only rewrite never performs.

set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"
PUBLIC_REPO="RBLN-SW/rbln-container-toolkit"
DRY_RUN="${MIRROR_DRY_RUN:-0}"

# RC tags should not be mirrored
if [[ "${TAG}" == *-rc* ]]; then
    echo "==> RC tag detected (${TAG}), skipping mirror push"
    exit 0
fi

HACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SANITIZE="${HACK_DIR}/sanitize-commit-message.sh"
SRC_REPO="$(git rev-parse --show-toplevel)"
SRC_HEAD="$(git rev-parse HEAD)"

if ! git rev-parse -q --verify "refs/tags/${TAG}^{commit}" >/dev/null; then
    echo "::error::tag ${TAG} does not exist in this checkout"
    exit 1
fi

SCRATCH="$(mktemp -d)"
cleanup() {
    rm -f ~/.ssh/deploy_key
    rm -rf "${SCRATCH}"
}
trap cleanup EXIT

echo "==> Rewriting history for ${PUBLIC_REPO}..."
# Work in a scratch clone: filter-branch rewrites refs in place and leaves
# refs/original/ backups behind, none of which may leak into the caller's
# checkout. The clone carries every tag plus HEAD, which is all we need.
MIRROR_REPO="${SCRATCH}/repo"
git clone -q --no-hardlinks "${SRC_REPO}" "${MIRROR_REPO}"
git -C "${MIRROR_REPO}" checkout -q -B mirror "${SRC_HEAD}"

# Rewrite `mirror` and every tag reachable into the same history. Tags that
# point at rewritten commits are re-created (`--tag-name-filter cat` keeps
# their names; annotated tags stay annotated, signatures are dropped).
export FILTER_BRANCH_SQUELCH_WARNING=1
git -C "${MIRROR_REPO}" filter-branch -f \
    --msg-filter "'${SANITIZE}'" \
    --tag-name-filter cat \
    -- mirror --tags >/dev/null

# The sanitizer must be a fixed point on the rewritten history: if running it
# again would still change a message, a reference slipped through and the
# pattern list needs extending. Fail loudly rather than publish it.
echo "  Verifying no internal references remain..."
leftovers=0
while read -r commit; do
    original="$(git -C "${MIRROR_REPO}" log -1 --format=%B "${commit}" | git stripspace)"
    resanitized="$(printf '%s' "${original}" | "${SANITIZE}")"
    if [[ "${original}" != "${resanitized}" ]]; then
        echo "::error::commit ${commit} still carries an internal reference after rewrite:"
        diff <(printf '%s\n' "${original}") <(printf '%s\n' "${resanitized}") || true
        leftovers=$((leftovers + 1))
    fi
done < <(git -C "${MIRROR_REPO}" rev-list mirror)
if (( leftovers > 0 )); then
    echo "::error::${leftovers} commit(s) not fully sanitized; extend hack/sanitize-commit-message.sh"
    exit 1
fi

# Every stable tag rides along so the public tags follow the rewritten
# history; RC tags are never published.
mapfile -t STABLE_TAGS < <(git -C "${MIRROR_REPO}" tag -l 'v*' | grep -v -- '-rc' || true)
TAG_REFSPECS=()
for t in "${STABLE_TAGS[@]}"; do
    TAG_REFSPECS+=("refs/tags/${t}:refs/tags/${t}")
done

# Messages are the only thing that may change: the published tree must be
# byte-identical to the internal one, or the mirror is shipping different code.
MIRROR_HEAD="$(git -C "${MIRROR_REPO}" rev-parse mirror)"
if [[ "$(git -C "${MIRROR_REPO}" rev-parse "${MIRROR_HEAD}^{tree}")" != "$(git rev-parse "${SRC_HEAD}^{tree}")" ]]; then
    echo "::error::rewritten HEAD tree differs from internal HEAD tree; refusing to mirror"
    exit 1
fi
TAG_COMMIT="$(git -C "${MIRROR_REPO}" rev-parse "refs/tags/${TAG}^{commit}")"
if [[ "$(git -C "${MIRROR_REPO}" rev-parse "${TAG_COMMIT}^{tree}")" != "$(git rev-parse "refs/tags/${TAG}^{tree}")" ]]; then
    echo "::error::rewritten ${TAG} tree differs from the internal tag's tree; refusing to mirror"
    exit 1
fi
echo "  internal HEAD ${SRC_HEAD} -> public main ${MIRROR_HEAD}"
TAG_KIND="lightweight"
[[ "$(git -C "${MIRROR_REPO}" cat-file -t "refs/tags/${TAG}")" == "tag" ]] && TAG_KIND="annotated"
echo "  ${TAG} (${TAG_KIND}) -> ${TAG_COMMIT}"

if [[ "${DRY_RUN}" == "1" ]]; then
    echo "==> Dry run: would force-push main and ${#STABLE_TAGS[@]} stable tag(s) (${STABLE_TAGS[*]})"
    echo "  Rewritten history (newest 5):"
    git -C "${MIRROR_REPO}" log --oneline -5 mirror | sed 's/^/    /'
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

# Pre-populate known_hosts with GitHub's published fingerprint
ssh-keyscan -H github.com >> ~/.ssh/known_hosts 2>/dev/null
export GIT_SSH_COMMAND="ssh -i ~/.ssh/deploy_key -o UserKnownHostsFile=${HOME}/.ssh/known_hosts"
REMOTE_URL="git@github.com:${PUBLIC_REPO}.git"

git -C "${MIRROR_REPO}" remote add public "${REMOTE_URL}"

echo "  Pushing main branch..."
# One-way mirror: public main is overwritten unconditionally. --force-with-lease
# would reject the first-ever push (no remote-tracking ref + unrelated history
# on the seeded public repo) without providing real safety here, since this
# script is the sole writer to the public main.
git -C "${MIRROR_REPO}" push public --force mirror:main

echo "  Pushing ${#STABLE_TAGS[@]} stable tag(s) incl. ${TAG}..."
git -C "${MIRROR_REPO}" push public --force "${TAG_REFSPECS[@]}"

echo "==> Mirror push complete"
