#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GOPOD Custom Installation Wizard
# Interactive or parameterized installer: select release tag (latest, dev, sha),
# deployment mode (container or quadlet), custom ports, and directories.
# https://github.com/ijon6k/gopod
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
INSTALL_URL="https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh"

echo -e "\033[0;36m==>\033[0m \033[1mStarting GOPOD Custom Installation Wizard...\033[0m"

# Forward to install.sh with interactive flag
if [ -n "$SCRIPT_DIR" ] && [ -f "${SCRIPT_DIR}/install.sh" ]; then
    if [ -t 0 ] || [ -e /dev/tty ]; then
        exec "${SCRIPT_DIR}/install.sh" --interactive "$@" < /dev/tty
    else
        exec "${SCRIPT_DIR}/install.sh" --interactive "$@"
    fi
else
    # Piped curl execution: fetch master install.sh and run interactively with TTY attached
    if [ -t 0 ] || [ -e /dev/tty ]; then
        exec curl -fsSL "$INSTALL_URL" | bash -s -- --interactive "$@" < /dev/tty
    else
        exec curl -fsSL "$INSTALL_URL" | bash -s -- --interactive "$@"
    fi
fi
