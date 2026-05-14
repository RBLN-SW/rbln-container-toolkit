# Vendored: github.com/RBLN-SW/go-rbln-ml

This directory is a verbatim copy of [RBLN-SW/go-rbln-ml](https://github.com/RBLN-SW/go-rbln-ml)
brought in-tree so the container toolkit can be built and tested without
configuring access to the upstream repository (which is private at the
time of this snapshot).

| Field | Value |
|---|---|
| Source | `https://github.com/RBLN-SW/go-rbln-ml` |
| Commit | `68b7042e8d3ae202da8cfce8cf94a95754bec81d` |
| Imported on | 2026-05-13 |
| Wire-up | `replace github.com/RBLN-SW/go-rbln-ml => ./third_party/go-rbln-ml` in the top-level `go.mod` |

## Licensing

The upstream snapshot at commit `68b7042e` does not carry a `LICENSE` /
`COPYING` file. Both `RBLN-SW/go-rbln-ml` and this container toolkit
(`rebellions-sw/ssw-rbln-container-toolkit`, mirrored publicly as
`RBLN-SW/rbln-container-toolkit`) are owned by the same legal entity
(Rebellions Inc.), and the toolkit is released under the
[Apache License 2.0](../../LICENSE) per the repository's top-level
`LICENSE` file. The vendored tree is redistributed in-tree under that
same Apache-2.0 license.

When the upstream repository publishes its own license file, the next
re-sync (see "When to update" below) should pick it up and this section
should be revised to defer to that authoritative license text.

The `replace` directive routes every `import "github.com/RBLN-SW/go-rbln-ml/..."`
through this local copy, so consumers of the toolkit pull no extra
dependencies — and CI builders don't need credentials to fetch the
upstream module.

## When to update

Re-sync this directory whenever the toolkit needs new rblnml features
(new public functions in the C ABI, additional struct fields, etc.).
The procedure is:

```bash
# 1. Pick the upstream commit to snapshot.
PIN_SHA=<commit>

# 2. Clone fresh into a scratch dir.
git clone https://github.com/RBLN-SW/go-rbln-ml /tmp/go-rbln-ml-src
git -C /tmp/go-rbln-ml-src checkout "${PIN_SHA}"

# 3. Replace this tree with the new snapshot (preserving VENDORED.md).
rsync -a --delete --exclude='.git' --exclude='VENDORED.md' \
    /tmp/go-rbln-ml-src/ third_party/go-rbln-ml/

# 4. Bump the metadata table above (commit + import date), then
#    rebuild the rblnml-flavored binaries to confirm the bump didn't
#    break the cgo binding surface used by internal/topology/rblnml.go.
make build-rblnml
make test-rblnml
```

When `go-rbln-ml` is open-sourced, drop this directory and the
`replace` line in `go.mod` — the toolkit will resume pulling the
module directly from the upstream registry.

## Modifications

### Patches

**Drop RSD group management API (`RsdGroupCreate`, `RsdGroupCreateAll`,
`RsdGroupDestroy`, `RsdGroupDestroyAll`)** — these wrappers reference
`rblnmlRsdGroup*` symbols that older `librbln-ml.so` builds don't ship.
Hosts running a pre-group-API driver therefore fail to link the toolkit
even though the resolver only needs `GetDeviceInfo`. Trimming the four
Go wrappers in `rblnml/rblnml.go` keeps the build flowing on those hosts
without losing any functionality the toolkit actually consumes.

The accompanying `test/rsd_group_test.go` and `example/main.go` were
removed in the same patch because both reference the dropped methods
and would no longer compile.

When upstream cuts a release that the toolkit can adopt as the new pin,
re-evaluate whether to bring these back — by then the minimum supported
driver version will hopefully include the group API.

### Dropped directories

| Path | Reason |
|---|---|
| `.git/` | Snapshot mechanics. |
| `example/` | Demonstrates `RsdGroup*` calls that were patched out (see above). |
| `test/` | Same reason. |
