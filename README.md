# RBLN Container Toolkit

<div align="center">

<img src="assets/rbln_logo.png" width="60%"/>

</div>

[![License](https://img.shields.io/github/license/rbln-sw/rbln-container-toolkit)](https://github.com/rbln-sw/rbln-container-toolkit/blob/main/LICENSE)

RBLN Container Toolkit enables container runtimes to access [Rebellions](https://rebellions.ai) NPU devices using the [Container Device Interface (CDI)](https://github.com/cncf-tags/container-device-interface) specification. It automatically discovers host RBLN libraries and tools, generates CDI specs, and configures your container runtime — so containers can use NPU hardware with zero application changes.

## How It Works

```
                          ┌─────────────────────────────────────────┐
  Host System             │           RBLN Container Toolkit        │
 ─────────────            │                                         │
                          │  1. Discover    RBLN libs & tools       │
  /usr/lib64/             │       ↓         on the host             │
    librbln-*.so ────────►│  2. Generate    CDI spec (rbln.yaml)    │
  /usr/bin/               │       ↓                                 │
    rbln-smi ────────────►│  3. Configure   container runtime       │
                          │       ↓         (containerd/crio/docker)│
                          │  4. Hook        update ldcache          │
                          │                 in containers            │
                          └──────────────────────┬──────────────────┘
                                                 │
                                                 ▼
                          ┌──────────────────────────────────────────┐
  Container               │  $ docker run --device rebellions.ai/    │
                          │      npu=all my-app  # or npu=0          │
                          │                                          │
                          │  ✓ RBLN libraries mounted                │
                          │  ✓ Tools available (rbln-smi)            │
                          │  ✓ ldcache updated automatically         │
                          └──────────────────────────────────────────┘
```

The toolkit provides three binaries:

| Binary | Role |
|--------|------|
| **`rbln-ctk`** | Main CLI — generate CDI specs, configure runtimes, inspect system |
| **`rbln-ctk-daemon`** | Kubernetes daemon — automated setup with health endpoints and graceful shutdown |
| **`rbln-cdi-hook`** | OCI hook — runs inside containers to update ldcache and create symlinks |

## Features

- **Automatic Discovery** — Finds RBLN libraries, their dependencies, and CLI tools on the host
- **Multi-Runtime** — Supports containerd, CRI-O, and Docker
- **Library Isolation** — Optional isolated library paths to prevent host/container conflicts
- **Kubernetes Native** — DaemonSet deployment with liveness, readiness, and startup probes
- **CoreOS / OpenShift** — Works with driver containers via `--driver-root`
- **SELinux** — Configurable mount context for enforcing systems
- **Dry Run** — Preview all changes before applying

## Prerequisites

- Linux x86_64 (Ubuntu 22.04+, RHEL 9+, or Red Hat CoreOS)
- [RBLN driver](https://rebellions.ai) installed on the host
- A supported container runtime: containerd, CRI-O, or Docker

## Installation

### From Package (Recommended)

```bash
# Debian/Ubuntu
sudo dpkg -i rbln-container-toolkit_<version>_amd64.deb

# RHEL/CentOS
sudo rpm -i rbln-container-toolkit-<version>.x86_64.rpm
```

The package installs all three binaries and systemd units automatically.

### From Source

```bash
git clone https://github.com/RBLN-SW/rbln-container-toolkit.git
cd rbln-container-toolkit
make build           # pure-Go static binaries, no host dependencies
sudo make install
```

#### Build flavors

The toolkit ships in two flavors that target different deployments:

| Flavor | Make target | Distribution artifact | RSD auto-attach |
|---|---|---|---|
| **Stub (pure-Go)** | `make build` | Docker image (`deployments/container/Dockerfile`) — what Kubernetes DaemonSet deployments pull. | Off. The daemon's K8s path (`Devices.Disabled=true` on containerd/CRI-O) hands RSD ownership to the device-plugin and `setup.resolveTopology` short-circuits to `NoopResolver`, so `librbln-ml` is never called. |
| **rblnml (cgo)** | `make build-rblnml` | DEB / RPM (`make package-deb`, `make package-rpm`) — for standalone Docker hosts where operators run `rbln-ctk cdi generate` directly. | On. Each spec generation queries `librbln-ml` once to record every NPU's GroupID and attaches the resolved `/dev/rsdM` to per-NPU CDI entries. |

Host requirements:

| Flavor | Build host | Runtime host |
|---|---|---|
| Stub | Go ≥ 1.24 | nothing (binary is static, no host libraries) |
| rblnml | Go ≥ 1.24, `librbln-ml` + headers (the Rebellions UMD/driver package ships both) | `librbln-ml.so` resolvable by `ld.so` — DEB/RPM declares `librbln-ml` as a package dependency to enforce this |

The Go bindings (`go-rbln-ml`) ship in-tree under
[`third_party/`](third_party/go-rbln-ml/VENDORED.md) so neither flavor needs
external GitHub credentials to build the toolkit.

`make test` and `make test-rblnml` follow the same split — the stub-flavor
test suite is what CI runs by default, and `test-rblnml` exercises the cgo
path when the driver is available locally.

## Quick Start

The fastest way to get NPU access in containers:

```bash
# 1. Generate CDI specification (discovers RBLN libraries on host)
sudo rbln-ctk cdi generate

# 2. Configure your container runtime for CDI support
sudo rbln-ctk runtime configure

# 3. Run a container with NPU access
docker run --device rebellions.ai/npu=all -it ubuntu:22.04            # all NPUs
docker run --device rebellions.ai/npu=0 -it ubuntu:22.04              # NPU 0 only
docker run --device rebellions.ai/npu=0 --device rebellions.ai/npu=1 \
  -it ubuntu:22.04                                                    # NPU 0 + 1
```

That's it. The toolkit auto-detects your runtime and applies the right configuration.

### Verify Setup

```bash
# Check what was discovered
rbln-ctk cdi list

# View system info
rbln-ctk info

# Use NPU tools inside a container
docker run --device rebellions.ai/npu=all -it ubuntu:22.04 rbln-smi
```

### Preview Before Applying

Every command supports `--dry-run` to see what would change without modifying anything:

```bash
rbln-ctk cdi generate --dry-run
rbln-ctk runtime configure --dry-run
```

## User Guide

### Standalone Setup (rbln-ctk)

For bare-metal or VM hosts running containers directly.

#### Step 1: Generate CDI Spec

```bash
sudo rbln-ctk cdi generate
```

This discovers RBLN libraries and tools, then writes a CDI spec to `/var/run/cdi/rbln.yaml`.

Options:

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output` | Output path | `/var/run/cdi/rbln.yaml` |
| `-f, --format` | Output format (`yaml` or `json`) | `yaml` |
| `--driver-root` | Root path for driver files (CoreOS: `/host`) | `/` |
| `--container-library-path` | Isolated library path in container | _(same as host)_ |
| `--dry-run` | Preview without writing | `false` |

#### Step 2: Configure Runtime

```bash
sudo rbln-ctk runtime configure
```

Auto-detects the running container runtime and enables CDI support in its configuration.

> **Docker hosts:** Docker ships an embedded containerd whose socket is also present, so a Docker host exposes both sockets. Auto-detection refuses to guess when more than one runtime is found and asks you to disambiguate — pass `-r docker` explicitly:
>
> ```bash
> sudo rbln-ctk runtime configure -r docker
> ```

| Flag | Description | Default |
|------|-------------|---------|
| `-r, --runtime` | Force specific runtime (`containerd`, `crio`, `docker`) | _(auto-detect)_ |
| `--config-path` | Custom runtime config path | _(runtime default)_ |
| `--dry-run` | Preview changes | `false` |

#### Step 3: Restart Runtime

The runtime must be restarted to pick up the new configuration:

```bash
sudo systemctl restart containerd  # or crio, docker
```

#### Step 4: Run Containers

Each generated spec exposes one CDI entry per NPU plus a few group handles:

| Entry | What gets mounted |
|---|---|
| `rebellions.ai/npu=N` | `/dev/rblnN` plus the RSD group device the NPU is assigned to. The host's `/dev/rsdM` is exposed inside the container as **`/dev/rsd0`** regardless of `M`, because the UMD only ever opens `/dev/rsd0`. Auto-attachment requires the `with_rblnml` build (links against `librbln-ml`); the default pure-Go build leaves the entry NPU-only and logs a warning at spec generation so operators can add `--device rebellions.ai/npu=rsdM` explicitly or rebuild with the tag. |
| `rebellions.ai/npu=rsdM` | Host `/dev/rsdM`, exposed inside the container as `/dev/rsd0` — for explicit group selection (custom-group setups, debugging, or as a workaround in the pure-Go build). |
| `rebellions.ai/npu=all` | Every discovered `/dev/rbln*` and `/dev/rsd*`, each under its own host name (no `/dev/rsd0` renaming — two groups would collide). Use this when you want to expose the whole host. |
| `rebellions.ai/npu=runtime` | v0.1.x compatibility alias of `=all` (identical content). Prefer `=all` for new manifests. |

> **One RSD group per container.** Because every group device is renamed to
> `/dev/rsd0`, a container may only hold NPUs from a single RSD group. Selecting
> NPUs from two groups (e.g. `npu=0` and `npu=4` after `rbln-smi group -c 1 -a
> 4,5,6,7`) makes both entries claim `/dev/rsd0` and only one wins. Run one
> container per group, or use `npu=all` when you really need the whole host.

```bash
# Docker — single NPU
docker run --device rebellions.ai/npu=0 -it ubuntu:22.04

# Docker — multi NPU
docker run --device rebellions.ai/npu=0 --device rebellions.ai/npu=1 \
  -it ubuntu:22.04

# Docker — NPUs from a second RSD group (`rbln-smi group -c 1 -a 4,5,6,7`).
# The group's /dev/rsd1 shows up as /dev/rsd0 inside the container; no extra
# `--device /dev/rsd1:/dev/rsd0` flag is needed.
docker run --device rebellions.ai/npu=4 --device rebellions.ai/npu=5 \
  --device rebellions.ai/npu=6 --device rebellions.ai/npu=7 \
  -it ubuntu:22.04

# Docker — all NPUs (`npu=runtime` still works as a v0.1.x compat alias)
docker run --device rebellions.ai/npu=all -it ubuntu:22.04
```

```yaml
# Kubernetes Pod
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: ubuntu:22.04
    resources:
      limits:
        rebellions.ai/npu: "1"
```

#### RDS char device (`/dev/rblnfs`)

The RDS (Rebellions Datastore) char device `/dev/rblnfsN` is injected through a
**separate, opt-in CDI class** — `rebellions.ai/rds` — written to its own spec
file (`/var/run/cdi/rbln-rds.yaml`). It is deliberately kept out of the
`rebellions.ai/npu` class so the NPU `=all` selection never carries it, and it is
emitted **independently of the Kubernetes device-node gate**: because the device
is only injected into containers that explicitly reference it, it never masks
device-plugin NPU allocations.

| Entry | What gets injected |
|---|---|
| `rebellions.ai/rds=rblnfsN` | `/dev/rblnfsN` char device node (with the matching device-cgroup `rw` rule). |
| `rebellions.ai/rds=all` | Every discovered `/dev/rblnfs*` node. |

```bash
# Docker — inject the RDS char device (opt-in)
docker run --device rebellions.ai/rds=all -it ubuntu:22.04
docker run --device rebellions.ai/rds=rblnfs0 -it ubuntu:22.04
```

```yaml
# Kubernetes Pod — opt-in via CDI annotation (no device-plugin involvement).
# Requires containerd 1.7+ (enable_cdi + cdi_spec_dirs) or CRI-O 1.23.2+ (CDI on
# by default; cdi_spec_dirs must include the daemon's --cdi-spec-dir, which the
# default /var/run/cdi already does). Older CRI-O has no CDI support.
apiVersion: v1
kind: Pod
metadata:
  annotations:
    cdi.k8s.io/rblnfs: rebellions.ai/rds=rblnfs0
spec:
  containers:
  - name: app
    image: ubuntu:22.04
```

> Pods that do **not** carry the annotation receive no `/dev/rblnfs*`, and NPU
> (`/dev/rbln*`) classification/injection is unaffected.

### Kubernetes Deployment (rbln-ctk-daemon)

For Kubernetes clusters, deploy as a DaemonSet. The daemon handles the entire lifecycle:

1. Generates CDI spec on startup
2. Configures the container runtime **only if CDI isn't already enabled**
3. Restarts the runtime **only when it changed the config** (step 2)
4. Serves health check endpoints
5. Removes the CDI spec files on SIGTERM (pod termination)

> **Runtime restarts are conditional.** The runtime rescans the CDI spec dir
> at container-creation time, so refreshing specs never needs a restart —
> only enabling CDI in the runtime config does. If CDI is already enabled
> (e.g. containerd 2.0+ / Docker 28.2+ where it is on by default, or a node
> the daemon already configured), redeploying the DaemonSet performs **no**
> runtime restart. A node with CDI initially disabled is restarted once, on
> the first deploy.

> **Shutdown does not revert the runtime config.** On SIGTERM the daemon
> removes only its CDI spec files; it does **not** restore the runtime config
> from `.backup` or restart the runtime. This is deliberate — reverting would
> require another restart and re-arm the deploy/delete restart churn, and the
> left-in-place config lets the next deploy start up without a restart. As a
> result, uninstalling the DaemonSet does **not** return the runtime config to
> its pre-CTK state. To explicitly revert a node (restore the backed-up config
> and restart the runtime), run the one-shot cleanup command, e.g.
> `rbln-ctk-daemon runtime containerd cleanup`.

#### Deploy

```bash
kubectl apply -f deployments/kubernetes/daemonset.yaml
```

#### Health Endpoints

| Endpoint | Probe Type | Returns 200 When |
|----------|------------|-------------------|
| `/live` | Liveness | Daemon process is running |
| `/ready` | Readiness | Setup is complete |
| `/startup` | Startup | Initialization finished |

#### Configuration via Environment

| Variable | Description | Default |
|----------|-------------|---------|
| `RBLN_CTK_DAEMON_RUNTIME` | Container runtime | _(auto-detect)_ |
| `RBLN_CTK_DAEMON_HOST_ROOT` | Host root mount path | `/` (host), `/host` (container) |
| `RBLN_CTK_DAEMON_DRIVER_ROOT` | Driver root path for CDI spec | `/` |
| `RBLN_CTK_DAEMON_CDI_SPEC_DIR` | CDI spec directory | `/var/run/cdi` |
| `RBLN_CTK_DAEMON_CONFIG_PATH` | Runtime config path override (final path — see below) | _(runtime default)_ |
| `RBLN_CTK_DAEMON_CONTAINER_LIBRARY_PATH` | Container library path for isolation | _(empty)_ |
| `RBLN_CTK_DAEMON_SOCKET` | Runtime socket path | _(auto-detect)_ |
| `RBLN_CTK_DAEMON_HEALTH_PORT` | Health check port | `8080` |
| `RBLN_CTK_DAEMON_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout | `30s` |
| `RBLN_CTK_DAEMON_PID_FILE` | PID file path | `/run/rbln/toolkit.pid` |
| `RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT` | Skip cleanup on exit | `false` |
| `RBLN_CTK_DAEMON_DEBUG` | Enable debug logging | `false` |
| `RBLN_CTK_DAEMON_FORCE` | Terminate existing instance before starting | `false` |

#### `RBLN_CTK_DAEMON_CONFIG_PATH` semantics

> In this section `CONFIG_PATH` refers to `RBLN_CTK_DAEMON_CONFIG_PATH`
> (CLI flag: `--config-path`), and `HOST_ROOT` refers to
> `RBLN_CTK_DAEMON_HOST_ROOT`.

When `CONFIG_PATH` is set, the daemon uses it **as the final path**
inside its own filesystem namespace and does **not** prefix `HOST_ROOT`
to it. The caller is responsible for making the file reachable and
writable at that exact path. The override must be an **absolute path**;
a relative value is rejected at startup.

When `CONFIG_PATH` is unset, the daemon falls back to the runtime
default (e.g. `/etc/containerd/config.toml`) and prefixes `HOST_ROOT`
so the host's config can be reached through the host-root bind mount.

| Scenario | `HOST_ROOT` | `CONFIG_PATH` | Path the daemon writes to |
|----------|-------------|---------------|---------------------------|
| Default on host | _(unset)_ | _(unset)_ | `/etc/containerd/config.toml` |
| DaemonSet, default config | `/host` | _(unset)_ | `/host/etc/containerd/config.toml` |
| rke2 / k3s on host-root mount | `/host` | `/host/var/lib/rancher/rke2/agent/etc/containerd/config.toml` | same as `CONFIG_PATH` |
| Operator-managed RW mount | `/host` | `/runtime/config-dir/config.toml` | same as `CONFIG_PATH` |

> ⚠️ **Breaking change (v0.2.0):** Prior versions auto-prefixed
> `RBLN_CTK_DAEMON_HOST_ROOT` to `RBLN_CTK_DAEMON_CONFIG_PATH`. If you
> were passing a host-relative path (rke2/k3s), prepend the
> `RBLN_CTK_DAEMON_HOST_ROOT` value manually, e.g.
> `/var/lib/rancher/rke2/...` → `/host/var/lib/rancher/rke2/...`.

#### CoreOS / OpenShift

For Red Hat CoreOS environments where the host filesystem is mounted at `/host`:

```yaml
env:
  - name: RBLN_CTK_DAEMON_HOST_ROOT
    value: "/host"
```

### Library Isolation

By default, RBLN libraries are bind-mounted at their host paths inside the container. If this causes conflicts (e.g., different glibc versions), use library isolation:

```bash
sudo rbln-ctk cdi generate --container-library-path /rbln/lib64
```

This mode:
- Mounts libraries to an isolated path (`/rbln/lib64`) instead of host paths
- Uses the CDI hook to run `ldconfig` inside the container at startup
- Avoids `LD_LIBRARY_PATH` — the ldcache handles library resolution natively
- Supports setuid binaries (which ignore `LD_LIBRARY_PATH`)

### Systemd Integration

For automatic CDI spec refresh when driver files change:

```bash
sudo cp deployments/systemd/rbln-cdi-refresh.service /etc/systemd/system/
sudo cp deployments/systemd/rbln-cdi-refresh.path /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rbln-cdi-refresh.path
```

## Configuration

The toolkit reads configuration from `/etc/rbln/container-toolkit.yaml`. A sample is provided in [`config/container-toolkit.yaml`](config/container-toolkit.yaml).

All CLI flags can also be set via environment variables with the prefix `RBLN_CTK_` (e.g., `--driver-root` becomes `RBLN_CTK_DRIVER_ROOT`).

Key configuration sections:

| Section | Controls |
|---------|----------|
| `cdi` | Output path, format, vendor/class names |
| `libraries` | Discovery patterns, plugin paths, container isolation path |
| `tools` | Which CLI tools to include (e.g., `rbln-smi`) |
| `search-paths` | Where to look for libraries and binaries |
| `glibc-exclude` | System libraries to exclude from CDI spec |
| `selinux` | SELinux mount context settings |
| `hooks` | CDI hook binary and ldconfig paths |

## Troubleshooting

### CDI spec not generated

```bash
# Verify RBLN driver is installed
ls /usr/lib64/librbln-*.so*

# Run with debug output
rbln-ctk cdi generate --debug

# Check what was discovered
rbln-ctk cdi list
```

### Container can't find RBLN libraries

```bash
# Verify hook is installed
ls -la /usr/local/bin/rbln-cdi-hook

# Regenerate CDI spec
sudo rbln-ctk cdi generate
```

### Runtime not picking up changes

```bash
# Restart the runtime after configuration
sudo systemctl restart containerd  # or crio, docker
```

### Permission errors

Most operations require root access:

```bash
sudo rbln-ctk cdi generate
sudo rbln-ctk runtime configure
```

## Supported Platforms

| OS | Architecture | Status |
|----|--------------|--------|
| Ubuntu 22.04+ | x86_64 | Supported |
| RHEL 9+ | x86_64 | Supported |
| Red Hat CoreOS | x86_64 | Supported |

## Development

```bash
make build    # Build all binaries
make test     # Run unit tests
make lint     # Run linter
make fmt      # Format code
```

See the [Makefile](Makefile) for the full list of targets including integration tests, E2E tests, and packaging.

## License

[Apache License 2.0](LICENSE)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `make test && make lint`
5. Submit a pull request
