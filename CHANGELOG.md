# RBLN Container Toolkit Changelog

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
