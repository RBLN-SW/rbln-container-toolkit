#!/bin/bash
# RBLN Container Toolkit - Pre-removal script

set -e

# Stop and disable systemd services if available
if command -v systemctl &> /dev/null; then
    systemctl stop rbln-cdi-refresh.path || true
    systemctl disable rbln-cdi-refresh.path || true
    systemctl stop rbln-cdi-refresh.service || true
fi

# Remove generated CDI specification
rm -f /etc/cdi/rbln.yaml || true
rm -f /var/run/cdi/rbln.yaml || true

echo "RBLN Container Toolkit removed."
