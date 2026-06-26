#!/usr/bin/env bash
set -euo pipefail

# Ensure the script is run with root/sudo privileges
if [ "$EUID" -ne 0 ]; then
    echo "Error: Please run this script as root or using sudo." >&2
    exit 1
fi

# Locate the compiled gophrdrv binary relative to script directory and current working directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_SOURCE=""

if [ -f "$SCRIPT_DIR/gophrdrv" ]; then
    BINARY_SOURCE="$SCRIPT_DIR/gophrdrv"
elif [ -f "$SCRIPT_DIR/bin/gophrdrv" ]; then
    BINARY_SOURCE="$SCRIPT_DIR/bin/gophrdrv"
elif [ -f "$SCRIPT_DIR/../bin/gophrdrv" ]; then
    BINARY_SOURCE="$SCRIPT_DIR/../bin/gophrdrv"
elif [ -f "./bin/gophrdrv" ]; then
    BINARY_SOURCE="./bin/gophrdrv"
elif [ -f "./gophrdrv" ]; then
    BINARY_SOURCE="./gophrdrv"
else
    echo "Error: gophrdrv binary not found. Please build the project first (e.g., run 'just build')." >&2
    exit 1
fi

# Install the binary to /usr/bin
echo "Installing gophrdrv to /usr/bin/gophrdrv..."
install -m 755 "$BINARY_SOURCE" /usr/bin/gophrdrv

echo "Installation completed successfully! You can now run 'gophrdrv' from anywhere."
