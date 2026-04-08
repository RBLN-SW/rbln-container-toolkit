# RBLN Container Toolkit Changelog

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
