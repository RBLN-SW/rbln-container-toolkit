#!/bin/bash
# RBLN Container Toolkit - Post-installation script

set -e

# Reload systemd if available
if command -v systemctl &> /dev/null; then
    systemctl daemon-reload || true

    # Enable and start the path unit for CDI refresh on driver changes
    systemctl enable rbln-cdi-refresh.path || true
    systemctl start rbln-cdi-refresh.path || true
fi

# Generate initial CDI specification
if [ -x /usr/bin/rbln-ctk ]; then
    echo "Generating initial CDI specification..."
    /usr/bin/rbln-ctk cdi generate --output /etc/cdi/rbln.yaml || true
fi

echo "RBLN Container Toolkit installed successfully."
echo "Run 'rbln-ctk --help' for usage information."
