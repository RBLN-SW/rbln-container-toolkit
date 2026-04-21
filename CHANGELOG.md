# RBLN Container Toolkit Changelog

## v0.2.0 (unreleased)

### Breaking Changes

- **`RBLN_CTK_DAEMON_CONFIG_PATH` is now treated as the final path** inside
  the daemon's filesystem — `HOST_ROOT` is no longer auto-prefixed when
  `CONFIG_PATH` is set. Callers that previously relied on the automatic
  prefix (e.g. rke2/k3s deployments passing a host-relative path) must
  prepend the `HOST_ROOT` value themselves.

  ```diff
  - RBLN_CTK_DAEMON_HOST_ROOT=/host
  - RBLN_CTK_DAEMON_CONFIG_PATH=/var/lib/rancher/rke2/agent/etc/containerd/config.toml
  + RBLN_CTK_DAEMON_HOST_ROOT=/host
  + RBLN_CTK_DAEMON_CONFIG_PATH=/host/var/lib/rancher/rke2/agent/etc/containerd/config.toml
  ```

  The previous behavior made it impossible to point `CONFIG_PATH` at an
  operator-managed RW volume mount that lives outside `HOST_ROOT`
  (the npu-operator layout), because the daemon always tried to write
  through the read-only `HOST_ROOT` mount. This change matches
  nvidia-container-toolkit's `nvidia-ctk-installer --config` semantics.

## v0.1.1

- Add device node discovery and CDI spec generation for RBLN NPU devices
- Add `RBLN_CTK_DAEMON_CONFIG_PATH` environment variable for runtime config override
- Add release workflow with script delegation pattern (tag-triggered CI/CD pipeline)

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
