#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GOPOD Beta / Development Preview Installer
# Automatically installs image tag: ghcr.io/ijon6k/gopod:dev
# https://github.com/ijon6k/gopod
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
INSTALL_URL="https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh"

echo -e "\033[0;36m==>\033[0m \033[1mLaunching GOPOD Beta / Development Preview installer (:dev)...\033[0m"

if [ -n "$SCRIPT_DIR" ] && [ -f "${SCRIPT_DIR}/install.sh" ]; then
    exec "${SCRIPT_DIR}/install.sh" --channel=dev "$@"
else
    # Piped curl execution: fetch master install.sh and run with dev channel
    if [ -t 0 ] || [ -e /dev/tty ]; then
        exec curl -fsSL "$INSTALL_URL" | bash -s -- --channel=dev "$@"
    else
        exec curl -fsSL "$INSTALL_URL" | bash -s -- --channel=dev -y "$@"
    fi
fi
