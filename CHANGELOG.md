# RBLN Container Toolkit Changelog

## v0.2.0

- **Auto-regenerate CDI spec on UMD driver upgrades**: the
  daemon now polls embedded `rbln version:` strings inside `librbln-*.so`
  and rewrites `/var/run/cdi/rbln.yaml` whenever it sees a change. Picks up
  driver re-install / upgrade transparently so newly started containers
  bind to the current libraries instead of the stale ones baked in at
  daemon start.
  - New `--refresh-interval` flag / `RBLN_CTK_DAEMON_REFRESH_INTERVAL`
    env var (default `60s`, `0` disables). Libraries are auto-discovered
    in the standard lib dirs, so operators don't need to configure paths.
  - CDI writes are now atomic (tmp + `fsync` + `rename`) so a regen in
    flight can never produce a torn read for the runtime reading the spec.
  - `/ready` health endpoint adds a `cdi-refresh` block reporting
    `last_run`, library count, and the last callback error if any.
  - Host-side `rbln-cdi-refresh.path` / `.service` units stay as-is; this
    only adds an in-daemon refresh path for DaemonSet deployments.
  - Probe scope is limited to UMD libraries that actually embed the
    `rbln version:` marker (`librbln-ccl`, `librbln-thunk`). `librbln-ml`
    ships without the marker, so a broader glob produced a perpetual
    `probe failed` warning on every tick without contributing any signal;
    both libraries flip versions in lockstep on driver upgrade so change
    detection stays sufficient.
- **Per-device NPU selection**: the generated CDI spec now exposes
  one entry per discovered NPU (`rebellions.ai/npu=0`, `=1`, ...), one entry
  per RSD group (`=rsd0`, `=rsd1`, ...), and an `=all` umbrella entry.
  `=runtime` is kept as a compatibility alias of `=all` with identical content
  so v0.1.x manifests and device-plugin builds keep matching the spec.
  Containers can opt in to a subset of the host's NPUs via
  `docker run --device rebellions.ai/npu=0 --device rebellions.ai/npu=1`.
  Multi-entry selection composes additively through CDI's standard merge rules.
- Library mounts, tool mounts, and ldcache/symlink hooks moved to the
  top-level `containerEdits` block so they apply to any `npu=*` selection
  without being duplicated per entry. Per-NPU entries carry their own
  `/dev/rbln{N}` plus the RSD group node the NPU belongs to (attached via the
  new `topology.RsdResolver` abstraction — see below). The Kubernetes path
  (`Devices.Disabled=true`) emits only the `all` library/tool handle with no
  device nodes; device-plugin / DRA continues to own per-Pod device
  injection.
- New `internal/topology` package introduces the `RsdResolver` interface that
  the CDI generator consults to attach the correct RSD group device to each
  per-NPU entry. The default `NoopResolver` reports "no mapping known" so a
  `--device rebellions.ai/npu=N` selection produces an NPU-only entry until
  the librbln-ml-backed resolver kicks in.
- Vendor the [go-rbln-ml](https://github.com/RBLN-SW/go-rbln-ml) Go bindings
  under `third_party/go-rbln-ml/` and wire them in via `replace` in `go.mod`.
  The upstream module is private at snapshot time, so vendoring lets every
  builder (CI runners, dev laptops, hardware QA servers) build the toolkit
  with no credentials configured. When the bindings ship publicly we drop
  the `third_party/` directory and the `replace` line; see
  `third_party/go-rbln-ml/VENDORED.md` for the resync procedure.
- Add a librbln-ml-backed `RsdResolver` behind the `with_rblnml` build tag.
  It walks `rblnmlDeviceGetCount` → `DeviceGetHandleByIndex` →
  `GetDeviceInfo` once per spec generation, caches the resulting NPU→GroupID
  map, and releases the device handles immediately — the daemon never holds
  `/dev/rbln*` opens between regen cycles. Builds without the tag (the
  default) fall through to `NoopResolver{}` with an operator-visible warning
  via `topology.LoadOrFallback`. Production builds for hosts with the
  Rebellions driver should use `go build -tags with_rblnml` and link against
  `librbln-ml`; a follow-up will flip the default build to use cgo + the
  tag once the CI/packaging story is settled.
- Build infrastructure & distribution split:
  - **Docker image** (`deployments/container/Dockerfile`) stays pure-Go
    (`CGO_ENABLED=0`). It targets Kubernetes DaemonSet deployments where
    `Devices.Disabled=true` already routes `setup.resolveTopology` through
    `NoopResolver`, so the cgo bindings would be dead weight — and the
    static image keeps no `librbln-ml.so` dependency.
  - **DEB / RPM packages** (`make package-deb`, `make package-rpm`) build
    with `make build-rblnml` (`CGO_ENABLED=1 -tags with_rblnml`). They target
    standalone Docker hosts where `rbln-ctk cdi generate` runs directly and
    operators rely on automatic NPU↔RSD mapping. The packages declare
    `librbln-ml` as a runtime dependency.
  - `make build` / `make test` defaults stay pure-Go (`CGO_ENABLED=0`, no
    build tags) for CI smoke tests and contributor laptops without a
    librbln-ml install. `make build-rblnml` / `make test-rblnml` (or
    `make build-rblnml-ci`, which builds the in-tree stub first) opt
    into the cgo path; the release pipeline uses `build-rblnml-ci` so
    shipped DEB/RPM binaries carry the librbln-ml NEEDED entry.
- `rebellions.ai/npu=runtime` keeps working as a v0.1.x compatibility alias
  of `=all` (same content, no manifest rewrite required). Prefer `=all` or a
  per-device selector for new manifests; the alias may be retired in a future
  release once downstream consumers have all migrated.

  Selector cheat sheet:

  | Selector | Effect |
  |---|---|
  | `--device rebellions.ai/npu=all` | All discovered NPUs + their RSD groups (recommended) |
  | `--device rebellions.ai/npu=runtime` | Same as `=all` — v0.1.x compatibility alias |
  | `--device rebellions.ai/npu=0 --device rebellions.ai/npu=1` | Two specific NPUs |
  | `--device rebellions.ai/npu=rsd2` | Explicit RSD group (custom topology) |

## v0.1.2

- Stop pinning host device nodes (`/dev/rbln*`, `/dev/rsd*`) into the runtime
  CDI device on Kubernetes runtimes. The daemon now sets
  `cfg.Devices.Disabled = true` whenever it targets containerd or CRI-O so that
  device-plugin / DRA owns per-Pod device injection. Without this fix
  `/dev/rsd0` was statically mounted into every Pod via
  `rebellions.ai/npu=runtime`, masking the RSD group device that
  device-plugin allocated for the workload.
- Add `devices.disabled` field to the toolkit config (default `false`,
  preserves Docker behavior of CTK injecting device nodes via the runtime CDI
  device). Setting it to `true` skips both device discovery and device-node
  emission in the generated spec.

## v0.1.1

- Add device node discovery and CDI spec generation for RBLN NPU devices
- Add `RBLN_CTK_DAEMON_CONFIG_PATH` environment variable for runtime config override
- Add release workflow with script delegation pattern (tag-triggered CI/CD pipeline)
- Mirror release image to Docker Hub (`rebellions/rbln-container-toolkit`) on stable tag publish
- `RBLN_CTK_DAEMON_CONFIG_PATH` is treated as the final path inside the
  daemon filesystem and is not auto-prefixed with `RBLN_CTK_DAEMON_HOST_ROOT`.
  Callers passing host-relative paths (rke2/k3s) must include the host-root
  prefix themselves, e.g. `/var/lib/rancher/rke2/...` →
  `/host/var/lib/rancher/rke2/...`. Matches `nvidia-ctk-installer --config`
  semantics.

## v0.1.0

- Initial open-source release of rbln-container-toolkit
- Add CDI spec generation for RBLN NPU devices (`rbln-ctk cdi generate`)
- Add CDI spec listing (`rbln-ctk cdi list`)
- Add runtime configuration for containerd, CRI-O, Docker (`rbln-ctk runtime configure`)
- Add OCI hook for container library injection (`rbln-cdi-hook`)
- Add `create-symlinks` and `update-ldcache` hook commands
- Add Kubernetes daemon mode with health checks (`rbln-ctk-daemon`)
- Add library and tool discovery from host system
- Add system info command (`rbln-ctk info`)
- Add container image and Kubernetes DaemonSet deployment manifests
- Add systemd units for CDI spec auto-refresh (`rbln-cdi-refresh.path`, `rbln-cdi-refresh.service`)
