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

# Locate completions directory relative to the script directory
COMPLETIONS_DIR=""
if [ -d "$SCRIPT_DIR/completions" ]; then
    COMPLETIONS_DIR="$SCRIPT_DIR/completions"
elif [ -d "$SCRIPT_DIR/../completions" ]; then
    COMPLETIONS_DIR="$SCRIPT_DIR/../completions"
fi

if [ -n "$COMPLETIONS_DIR" ]; then
    # Install bash completions
    BASH_COMPLETION_DIR="/usr/share/bash-completion/completions"
    if [ -d "$BASH_COMPLETION_DIR" ]; then
        echo "Installing bash completion to $BASH_COMPLETION_DIR/gophrdrv..."
        install -m 644 "$COMPLETIONS_DIR/gophrdrv.bash" "$BASH_COMPLETION_DIR/gophrdrv"
    else
        BASH_FALLBACK_DIR="/etc/bash_completion.d"
        if [ -d "$BASH_FALLBACK_DIR" ]; then
            echo "Installing bash completion to $BASH_FALLBACK_DIR/gophrdrv..."
            install -m 644 "$COMPLETIONS_DIR/gophrdrv.bash" "$BASH_FALLBACK_DIR/gophrdrv"
        else
            echo "Warning: Bash completion directory not found. Skipping bash completion install."
        fi
    fi

    # Install fish completions
    FISH_COMPLETION_DIR="/usr/share/fish/vendor_completions.d"
    if [ -d "$FISH_COMPLETION_DIR" ]; then
        echo "Installing fish completion to $FISH_COMPLETION_DIR/gophrdrv.fish..."
        install -m 644 "$COMPLETIONS_DIR/gophrdrv.fish" "$FISH_COMPLETION_DIR/gophrdrv.fish"
    else
        echo "Warning: Fish completion directory not found. Skipping fish completion install."
    fi
else
    echo "Warning: Completions directory not found. Skipping completion installation."
fi

echo "Installation completed successfully! You can now run 'gophrdrv' from anywhere."


