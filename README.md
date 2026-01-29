# RBLN Container Toolkit

<div align="center">

<img src="assets/rbln_logo.png" width="60%"/>

</div>

[![License](https://img.shields.io/github/license/rbln-sw/rbln-container-toolkit)](https://github.com/rbln-sw/rbln-container-toolkit/blob/main/LICENSE)

RBLN Container Toolkit enables GPU-like container support for Rebellions NPU devices using the Container Device Interface (CDI) specification.

## Features

- **CDI Specification Generation**: Automatically discovers RBLN libraries and tools, generating CDI specs for container runtimes
- **Multi-Runtime Support**: Configure containerd, CRI-O, and Docker for CDI support
- **Automated Installation**: One-command setup with automatic runtime restart
- **Library Isolation**: Optional isolated library paths to prevent conflicts with host libraries
- **CDI Hooks**: Automatic ldcache updates in containers for proper library resolution
- **CoreOS Support**: Works with Red Hat CoreOS and driver containers via `--driver-root`
- **Multi-OS Support**: Automatic detection and configuration for Ubuntu, RHEL, and CoreOS
- **SELinux Support**: Configurable mount context for SELinux-enabled systems

## Binaries

| Binary | Purpose |
|--------|---------|
| `rbln-ctk` | Main CLI tool for CDI generation and runtime configuration |
| `rbln-ctk-daemon` | Daemon mode for Kubernetes DaemonSet deployments with health checks and graceful shutdown |
| `rbln-cdi-hook` | CDI hook for updating ldcache in containers |

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/RBLN-SW/rbln-container-toolkit.git
cd rbln-container-toolkit/rbln-container-toolkit

# Build all binaries
make build

# Install (requires sudo)
sudo make install
```

### Binary Installation

```bash
sudo cp bin/rbln-ctk /usr/local/bin/
sudo cp bin/rbln-cdi-hook /usr/bin/
sudo cp bin/rbln-ctk-daemon /usr/local/bin/
sudo chmod +x /usr/local/bin/rbln-ctk /usr/local/bin/rbln-cdi-hook /usr/local/bin/rbln-ctk-daemon
```

## Quick Start

### Option 1: Using rbln-ctk (Recommended for standalone)

```bash
# Generate CDI specification
sudo rbln-ctk cdi generate

# Configure specific runtime
sudo rbln-ctk runtime configure --runtime containerd

# Preview changes without applying
rbln-ctk runtime configure --dry-run
```

### Option 2: Using rbln-ctk-daemon (Kubernetes DaemonSet)

The daemon mode is designed for Kubernetes deployments where it:
- Installs artifacts on startup
- Configures the container runtime
- Runs with health check endpoints (/live, /ready, /startup)
- Cleans up on SIGTERM

```bash
# Run as daemon (auto-detects runtime)
rbln-ctk-daemon

# Run with explicit runtime
rbln-ctk-daemon --runtime containerd

# Preview changes without applying
rbln-ctk-daemon --dry-run
```

### Option 3: Manual Setup

#### 1. Generate CDI Specification

```bash
# Generate CDI spec (requires RBLN driver installed)
sudo rbln-ctk cdi generate

# Preview without writing
rbln-ctk cdi generate --dry-run

# Output to stdout
rbln-ctk cdi generate --output -
```

#### 2. Configure Container Runtime

```bash
# Auto-detect and configure runtime
sudo rbln-ctk runtime configure

# Configure specific runtime
sudo rbln-ctk runtime configure --runtime containerd

# Preview changes
rbln-ctk runtime configure --dry-run
```

#### 3. Restart Runtime

```bash
# For containerd
sudo systemctl restart containerd

# For CRI-O
sudo systemctl restart crio

# For Docker
sudo systemctl restart docker
```

### 4. Run Container with NPU

```bash
# Docker
docker run --device rebellions.ai/npu=runtime -it ubuntu:22.04

# Kubernetes Pod spec
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

## Commands

### rbln-ctk - Main CLI

#### CDI Management

```bash
# Generate CDI specification
rbln-ctk cdi generate [flags]
  -o, --output              Output path (default: /var/run/cdi/rbln.yaml)
  -f, --format              Output format: yaml, json (default: yaml)
      --driver-root         Driver root path for CoreOS (default: /)
      --container-library-path  Container library path for isolation
      --dry-run             Preview without writing

# List discovered resources
rbln-ctk cdi list [flags]
  -f, --format              Output format: table, json, yaml
```

#### Runtime Configuration

```bash
# Configure container runtime for CDI
rbln-ctk runtime configure [flags]
  -r, --runtime             Runtime: containerd, crio, docker (auto-detected)
      --config-path         Custom config path
      --dry-run             Preview changes
```

#### System Information

```bash
# Display system info
rbln-ctk info

# Display version
rbln-ctk version
```

### rbln-ctk-daemon - Daemon Mode for Kubernetes

```bash
# Run as daemon (auto-detects runtime)
rbln-ctk-daemon [flags]
  -r, --runtime             Runtime: containerd, crio, docker (auto-detected)
      --shutdown-timeout    Graceful shutdown timeout (default: 30s)
      --pid-file            PID file path (default: /run/rbln/rbln-ctk-daemon.pid)
      --health-port         Health check endpoint port (default: 8080)
      --no-cleanup-on-exit  Skip cleanup on shutdown
      --host-root-mount     Host root mount for containerized deployment
      --cdi-spec-dir        CDI spec output directory
      --dry-run             Preview changes
```

Health endpoints:
- `/live` - Liveness probe (always returns 200 when running)
- `/ready` - Readiness probe (200 when setup complete)
- `/startup` - Startup probe (200 when initialized)

### rbln-cdi-hook - CDI Hook

```bash
# Update ldcache in container (called by CDI)
rbln-cdi-hook update-ldcache [flags]
      --folder              Library folders to add to ldcache
      --ldconfig-path       Path to ldconfig binary
      --container-spec      OCI container spec path
```

## Configuration

Configuration file location: `/etc/rbln/container-toolkit.yaml`

```yaml
# CDI specification settings
cdi:
  output-path: /var/run/cdi/rbln.yaml
  format: yaml
  vendor: rebellions.ai
  class: npu

# Library discovery
libraries:
  patterns:
    - "librbln-*.so*"
  plugin-paths:
    - /usr/lib64/libibverbs
    - /usr/lib/x86_64-linux-gnu/libibverbs
  container-path: ""  # Optional: isolated library path

# Tools to include
tools:
  - rbln-smi

# Search paths
search-paths:
  libraries:
    - /usr/lib64
    - /usr/lib/x86_64-linux-gnu
  binaries:
    - /usr/bin
    - /usr/local/bin

# SELinux settings (for RHEL/CoreOS)
selinux:
  enabled: false
  mount-context: "z"  # "z" for shared, "Z" for private

# Hook settings
hooks:
  path: /usr/local/bin/rbln-cdi-hook
  ldconfig-path: /sbin/ldconfig
```

### Environment Variables

#### rbln-ctk

| Variable | Description |
|----------|-------------|
| `RBLN_CTK_CONFIG` | Path to configuration file |
| `RBLN_CTK_DEBUG` | Enable debug logging |
| `RBLN_CTK_QUIET` | Suppress non-error output |
| `RBLN_CTK_OUTPUT` | CDI output path (for `cdi generate`) |
| `RBLN_CTK_FORMAT` | Output format for `cdi generate` (yaml/json) |
| `RBLN_CTK_DRIVER_ROOT` | Driver root path (for `cdi generate`) |
| `RBLN_CTK_CONTAINER_LIBRARY_PATH` | Container library path for isolation |
| `RBLN_CTK_LIST_FORMAT` | Output format for `cdi list` (table/json/yaml) |
| `RBLN_CTK_LIST_DRIVER_ROOT` | Driver root path (for `cdi list`) |
| `RBLN_CTK_RUNTIME` | Container runtime type |
| `RBLN_CTK_CONFIG_PATH` | Runtime config path |

#### rbln-ctk-daemon

| Variable | Description |
|----------|-------------|
| `RBLN_CTK_DAEMON_DEBUG` | Enable debug logging |
| `RBLN_CTK_DAEMON_HOST_ROOT` | Host root mount path (auto-detected: "/" on host, "/host" in container) |
| `RBLN_CTK_DAEMON_CDI_SPEC_DIR` | CDI spec directory |
| `RBLN_CTK_DAEMON_RUNTIME` | Container runtime (containerd/crio/docker) |
| `RBLN_CTK_DAEMON_HEALTH_PORT` | Health check port (default: 8080) |
| `RBLN_CTK_DAEMON_SHUTDOWN_TIMEOUT` | Shutdown timeout (default: 30s) |
| `RBLN_CTK_DAEMON_PID_FILE` | PID file path |
| `RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT` | Skip cleanup on exit (true/false) |

#### rbln-cdi-hook

| Variable | Description |
|----------|-------------|
| `RBLN_CDI_HOOK_FOLDER` | Library folders to add |
| `RBLN_CDI_HOOK_LDCONFIG_PATH` | ldconfig binary path |
| `RBLN_CDI_HOOK_CONTAINER_SPEC` | Container spec path |

## Library Isolation

For environments where host library conflicts may occur, use library isolation:

```bash
# Generate CDI spec with isolated library path
sudo rbln-ctk cdi generate --container-library-path /rbln/lib64

# Libraries are mounted to /rbln/lib64 and ldcache is updated via hook
```

This mode:
- Mounts RBLN libraries to a custom container path (e.g., `/rbln/lib64`)
- Uses CDI hooks to update ldcache in the container
- Avoids conflicts with host glibc and other system libraries

## CoreOS / OpenShift Deployment

For Red Hat CoreOS environments, deploy as a DaemonSet:

```bash
kubectl apply -f deployments/kubernetes/daemonset.yaml
```

The DaemonSet:
1. Runs `rbln-ctk-daemon` which auto-detects the container runtime
2. Generates CDI specification and configures the runtime
3. Provides health endpoints for Kubernetes probes
4. Gracefully cleans up on SIGTERM (pod termination)

Example DaemonSet environment configuration:
```yaml
env:
  - name: RBLN_CTK_DAEMON_HOST_ROOT
    value: "/host"
  - name: RBLN_CTK_DAEMON_CDI_SPEC_DIR
    value: "/var/run/cdi"
  - name: RBLN_CTK_DAEMON_HEALTH_PORT
    value: "8080"
```

## Systemd Integration

Install the systemd service for automatic CDI refresh:

```bash
sudo cp deployments/systemd/rbln-cdi-refresh.service /etc/systemd/system/
sudo cp deployments/systemd/rbln-cdi-refresh.path /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rbln-cdi-refresh.path
```

## Development

### Prerequisites

- Go 1.21+
- golangci-lint (for linting)

### Build

```bash
make build           # Build all binaries
make build-ctk       # Build rbln-ctk only
make build-hook      # Build rbln-cdi-hook only
make build-daemon    # Build rbln-ctk-daemon only
make test            # Run tests
make lint            # Run linter
make fmt             # Format code
make clean           # Clean build artifacts
```

### Testing

The toolkit includes comprehensive test coverage across unit, integration, and E2E tests.

#### Unit Tests

```bash
# Run all unit tests
make test

# Run with race detection
go test -race ./...

# Run with coverage report
make test-coverage

# Run specific package tests
go test -v ./internal/cdi/...
go test -v ./cmd/rbln-ctk/...

# Generate mock files (requires moq)
make generate
```

**Coverage Requirements**:
- Overall: >= 70%
- `cmd/rbln-cdi-hook`: >= 60%
- `cmd/rbln-ctk`: >= 40%
- `internal/installer`: >= 70%

#### Integration Tests

```bash
# Run integration tests
go test -v ./tests/integration/...

# Run with build tag
go test -tags=integration -v ./tests/integration/...
```

Integration tests cover:
- CDI lifecycle (generate → validate → list)
- Daemon lifecycle (start → health check → SIGTERM → cleanup)
- Runtime configuration changes

#### E2E Tests

E2E tests use Ginkgo framework with Docker containers to test in isolated environments.

```bash
# Install Ginkgo (first time only)
make ginkgo

# Run E2E tests locally (requires Docker)
make test-e2e-local

# Run E2E tests in container
make test-e2e

# Run on specific OS images
make test-e2e-ubuntu2204    # Ubuntu 22.04
make test-e2e-ubuntu2404    # Ubuntu 24.04
make test-e2e-rhel9         # RHEL 9

# Run on all supported OS images
make test-e2e-all-images

# Run full test suite (unit + integration + e2e)
make test-all

# Clean E2E artifacts
make clean-e2e
```

**E2E Test Structure**:
```
tests/e2e/
├── go.mod              # Separate module for Ginkgo dependencies
├── runner.go           # Test runner abstraction (local/container)
├── installer.go        # Toolkit installation in containers
├── e2e_suite_test.go   # Ginkgo test suite
├── cdi_test.go         # CDI generation tests
├── runtime_test.go     # Runtime configuration tests
├── docker_test.go      # Docker integration tests
└── cli_test.go         # CLI command tests
```

#### Test BDD Structure

All tests follow strict BDD (Behavior-Driven Development) structure:

```go
func TestExample(t *testing.T) {
    // Given - Setup only
    input := setupTestData()
    defer cleanup()
    
    // When - Single function call
    result, err := FunctionUnderTest(input)
    
    // Then - Assertions only
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Project Structure

```
rbln-container-toolkit/
├── cmd/
│   ├── rbln-ctk/              # Main CLI tool
│   ├── rbln-cdi-hook/         # CDI hook binary
│   └── rbln-ctk-daemon/       # Daemon for Kubernetes deployments
├── internal/
│   ├── cdi/                   # CDI generation and validation
│   ├── config/                # Configuration management
│   ├── daemon/                # Daemon lifecycle management
│   ├── discover/              # Library and tool discovery
│   ├── errors/                # Error types
│   ├── ldconfig/              # ldconfig wrapper
│   ├── oci/                   # OCI state management
│   ├── output/                # Output formatting
│   ├── restart/               # Runtime restart mechanisms
│   └── runtime/               # Runtime configuration
├── deployments/
│   ├── container/             # Dockerfile
│   ├── kubernetes/            # DaemonSet manifests
│   ├── systemd/               # Systemd units
│   └── scripts/               # Deployment scripts
├── config/                    # Sample configuration
└── tests/                     # Integration tests
```

## Supported Platforms

| OS | Architecture | Status |
|----|--------------|--------|
| Ubuntu 22.04+ | x86_64 | ✅ Supported |
| RHEL 9+ | x86_64 | ✅ Supported |
| Red Hat CoreOS | x86_64 | ✅ Supported |

## Troubleshooting

### CDI spec not generated

```bash
# Check if RBLN driver is installed
ls /usr/lib64/librbln-*.so*

# Run with debug output
rbln-ctk cdi generate --debug

# Check discovered libraries
rbln-ctk cdi list
```

### Runtime not restarting

```bash
# Restart manually after configuration
sudo systemctl restart docker

# Or for containerd
sudo systemctl restart containerd
```

### Permission denied errors

```bash
# Most operations require root
sudo rbln-ctk cdi generate
sudo rbln-ctk runtime configure
```

### Container can't find libraries

```bash
# Check if hook is installed
ls -la /usr/local/bin/rbln-cdi-hook

# Regenerate CDI spec
sudo rbln-ctk cdi generate
```

## License

Apache License 2.0

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `make test && make lint`
5. Submit a pull request

## Related Projects

- [CDI Specification](https://github.com/cncf-tags/container-device-interface)
