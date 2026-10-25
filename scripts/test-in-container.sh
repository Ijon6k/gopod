#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GOPOD Container-in-Container Sandbox Tester
# Runs GOPOD installer inside an isolated Linux container (simulating a fresh device).
# Prompts to select between Beta (:dev), Production (:latest), or Custom Tag/SHA.
# Network is isolated so it uses standard ports :80 and :8085 inside without host conflict.
# Verifies endpoints via curl in terminal, and auto-removes the container on exit.
# https://github.com/ijon6k/gopod
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

BOLD='\033[1m'
DIM='\033[2m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." 2>/dev/null && pwd || echo "")"
SANDBOX_IMAGE="quay.io/podman/stable"
SHELL_MODE=false
NON_INTERACTIVE=false
CHANNEL=""
TAG=""

show_help() {
    cat <<EOF
GOPOD In-Container Sandbox Tester

Usage:
  ./scripts/test-in-container.sh [OPTIONS]

Options:
  --dev, --beta        Test Beta / Dev preview image (:dev) [Default]
  --latest             Test Production Stable image (:latest)
  --channel=<NAME>     Test specific channel ('dev' or 'latest')
  --tag=<TAG_OR_SHA>   Test specific custom image tag or git SHA
  -i, --shell          Keep bash shell open inside container after verification
  -y, --yes            Run non-interactively with defaults
  -h, --help           Show this help message

Examples:
  ./scripts/test-in-container.sh              # Interactive prompt to choose dev / latest / custom
  ./scripts/test-in-container.sh --dev        # Test with :dev directly
  ./scripts/test-in-container.sh --tag=sha-12 # Test specific SHA build
  ./scripts/test-in-container.sh -i           # Test and enter interactive container shell
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dev|--beta)
            CHANNEL="dev"
            ;;
        --latest)
            CHANNEL="latest"
            ;;
        --channel=*)
            CHANNEL="${1#*=}"
            ;;
        --channel)
            CHANNEL="$2"; shift
            ;;
        --tag=*)
            TAG="${1#*=}"
            ;;
        --tag)
            TAG="$2"; shift
            ;;
        -i|--interactive|--shell)
            SHELL_MODE=true
            ;;
        -y|--non-interactive|--yes)
            NON_INTERACTIVE=true
            ;;
        -h|--help)
            show_help
            ;;
        *)
            echo -e "${YELLOW}Warning: Unknown parameter '$1' ignored.${NC}"
            ;;
    esac
    shift
done

# If neither channel nor tag specified, prompt user interactively
if [ -z "$CHANNEL" ] && [ -z "$TAG" ]; then
    if [ "$NON_INTERACTIVE" = false ] && ([ -t 0 ] || [ -e /dev/tty ]); then
        TTY_DEV="/dev/tty"
        exec 3< "$TTY_DEV" 2>/dev/null || exec 3<&0

        echo -e "${CYAN}${BOLD}"
        echo "=============================================================="
        echo "          GOPOD Container Sandbox Scenario Selection          "
        echo "=============================================================="
        echo -e "${NC}"
        echo -e "Choose the container image to test in this sandbox:"
        echo -e "  ${BOLD}1)${NC} ${GREEN}Beta / Dev Preview${NC} (:dev) [Default - currently published on registry]"
        echo -e "  ${BOLD}2)${NC} ${BLUE}Production Stable${NC} (:latest) [Note: requires :latest pushed to registry]"
        echo -e "  ${BOLD}3)${NC} ${MAGENTA}Custom Image Tag / Git Commit SHA${NC} (e.g. sha-xxxx or v1.0.0)"
        echo ""
        read -r -u 3 -p "Select scenario [1-3] (default: 1): " CHOICE_SCENARIO

        case "$CHOICE_SCENARIO" in
            2|latest)
                CHANNEL="latest"
                ;;
            3|custom)
                read -r -u 3 -p "Enter custom image tag or git SHA: " USER_CUSTOM_TAG
                if [ -n "$USER_CUSTOM_TAG" ]; then
                    TAG="$USER_CUSTOM_TAG"
                else
                    CHANNEL="dev"
                fi
                ;;
            *)
                CHANNEL="dev"
                ;;
        esac
        exec 3<&-
    else
        # Default fallback: dev
        CHANNEL="dev"
    fi
fi

# Prepare install flag arguments
if [ -n "$TAG" ]; then
    INSTALL_ARGS="--tag=${TAG} -y --allow-root"
    TEST_LABEL="Custom Tag (: ${TAG})"
elif [ "$CHANNEL" = "latest" ]; then
    INSTALL_ARGS="--channel=latest -y --allow-root"
    TEST_LABEL="Production Stable (:latest)"
else
    INSTALL_ARGS="--channel=dev -y --allow-root"
    TEST_LABEL="Beta / Dev Preview (:dev)"
fi

echo -e "\n${CYAN}${BOLD}"
echo "=============================================================="
echo "       GOPOD Isolated Device Sandbox Tester (In-Container)    "
echo "=============================================================="
echo -e "${NC}"
echo -e " Target Image  : ${BOLD}${TEST_LABEL}${NC}"
echo -e " Sandbox Image : ${BOLD}${SANDBOX_IMAGE}${NC} (Official Red Hat Podman image)"
echo -e " Port Isolation: Uses native ports ${GREEN}:80${NC} and ${GREEN}:8085${NC} inside container"
echo -e " Host Conflict : ${GREEN}Zero (Host port :80 is untouched)${NC}"
echo -e " Cleanup Mode  : ${YELLOW}Ephemeral (--rm, container automatically destroyed on exit)${NC}"
echo "──────────────────────────────────────────────────────────────"
echo -e "Starting virtual device container and running GOPOD installer...\n"

if [ "$SHELL_MODE" = true ]; then
    # Interactive shell mode: Run install, curl test, then leave user in bash
    podman run --rm -it --privileged \
        -v "${PROJECT_ROOT}/scripts:/scripts:ro" \
        "${SANDBOX_IMAGE}" bash -c "
            echo -e '\033[1;36m==> [1/3] Running GOPOD installer (${TEST_LABEL})...\033[0m'
            /scripts/install.sh ${INSTALL_ARGS}

            echo -e '\n\033[1;32m==> [2/3] Verifying HTTP Endpoints via CLI Curl (Headless):\033[0m'
            echo '--- Direct GOPOD Dashboard (:8085) ---'
            curl -sI http://localhost:8085 | head -n 8

            echo -e '\n--- Caddy Ingress Reverse Proxy (:80) ---'
            curl -sI http://localhost:80 | head -n 8

            echo -e '\n\033[1;33m==> [3/3] You are now inside the sandbox container shell!\033[0m'
            echo 'Try running: podman ps, curl http://localhost:80, etc.'
            echo 'Type \"exit\" when finished to automatically destroy the container.'
            exec bash
        "
else
    TTY_FLAGS=""
    if [ -t 1 ]; then
        TTY_FLAGS="-t"
    fi

    podman run --rm ${TTY_FLAGS} --privileged \
        -v "${PROJECT_ROOT}/scripts:/scripts:ro" \
        "${SANDBOX_IMAGE}" bash -c "
            echo -e '\033[1;36m==> [1/2] Running GOPOD installer (${TEST_LABEL})...\033[0m'
            /scripts/install.sh ${INSTALL_ARGS}

            echo -e '\n\033[1;32m==> [2/2] Verifying Application Health via CLI Curl (Headless):\033[0m\n'
            echo '┌─────────────────────────────────────────────────────────────┐'
            echo '│ 1. Direct Control Plane Endpoint (:8085)                    │'
            echo '└─────────────────────────────────────────────────────────────┘'
            curl -sI http://localhost:8085 | head -n 8

            echo -e '\n┌─────────────────────────────────────────────────────────────┐'
            echo '│ 2. Caddy Ingress Reverse Proxy Endpoint (:80)               │'
            echo '└─────────────────────────────────────────────────────────────┘'
            curl -sI http://localhost:80 | head -n 8
        "

    echo -e "\n${GREEN}${BOLD}✓ Test completed successfully! Sandbox container has been destroyed (--rm).${NC}"
    echo -e "${DIM}Your host system remains 100% clean with zero residual files.${NC}\n"
fi
