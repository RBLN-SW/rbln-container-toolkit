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
                          │      npu=runtime my-app                  │
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
make build
sudo make install
```

## Quick Start

The fastest way to get NPU access in containers:

```bash
# 1. Generate CDI specification (discovers RBLN libraries on host)
sudo rbln-ctk cdi generate

# 2. Configure your container runtime for CDI support
sudo rbln-ctk runtime configure

# 3. Run a container with NPU access
docker run --device rebellions.ai/npu=runtime -it ubuntu:22.04
```

That's it. The toolkit auto-detects your runtime and applies the right configuration.

### Verify Setup

```bash
# Check what was discovered
rbln-ctk cdi list

# View system info
rbln-ctk info

# Use NPU tools inside a container
docker run --device rebellions.ai/npu=runtime -it ubuntu:22.04 rbln-smi
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

```bash
# Docker
docker run --device rebellions.ai/npu=runtime -it ubuntu:22.04
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

### Kubernetes Deployment (rbln-ctk-daemon)

For Kubernetes clusters, deploy as a DaemonSet. The daemon handles the entire lifecycle:

1. Generates CDI spec on startup
2. Configures the container runtime
3. Restarts the runtime
4. Serves health check endpoints
5. Cleans up on SIGTERM (pod termination)

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
| `RBLN_CTK_DAEMON_CONTAINER_LIBRARY_PATH` | Container library path for isolation | _(empty)_ |
| `RBLN_CTK_DAEMON_SOCKET` | Runtime socket path | _(auto-detect)_ |
| `RBLN_CTK_DAEMON_HEALTH_PORT` | Health check port | `8080` |
| `RBLN_CTK_DAEMON_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout | `30s` |
| `RBLN_CTK_DAEMON_PID_FILE` | PID file path | `/run/rbln/toolkit.pid` |
| `RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT` | Skip cleanup on exit | `false` |
| `RBLN_CTK_DAEMON_DEBUG` | Enable debug logging | `false` |
| `RBLN_CTK_DAEMON_FORCE` | Terminate existing instance before starting | `false` |

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
