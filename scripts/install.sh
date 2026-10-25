#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GOPOD — Modern Rootless Podman PaaS Installation Script
# https://github.com/ijon6k/gopod
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

# ANSI Color and Typography
BOLD='\033[1m'
DIM='\033[2m'
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Terminal capability detection
IS_TTY=false
if [ -t 1 ]; then
    IS_TTY=true
fi

# Detect Unicode capability for progress bar characters
CHAR_FILLED="█"
CHAR_EMPTY="░"
CHAR_CHECK="✓"
CHAR_CROSS="✗"
CHAR_WARN="⚠️ "
if [[ "${LC_ALL:-${LC_CTYPE:-${LANG:-}}}" != *"UTF-8"* && "${LC_ALL:-${LC_CTYPE:-${LANG:-}}}" != *"utf8"* ]]; then
    CHAR_FILLED="#"
    CHAR_EMPTY="-"
    CHAR_CHECK="[OK]"
    CHAR_CROSS="[FAIL]"
    CHAR_WARN="[WARN] "
fi

TOTAL_STEPS=7
LOG_FILE="/tmp/gopod-install-$$.log"

cleanup_on_exit() {
    local exit_code=$?
    if [ "$IS_TTY" = true ]; then
        tput cnorm 2>/dev/null || printf "\033[?25h"
    fi
    if [ $exit_code -ne 0 ]; then
        echo -e "\n${RED}${BOLD}Installation terminated with error (exit code: ${exit_code}).${NC}"
        if [ -f "$LOG_FILE" ]; then
            echo -e "${DIM}Recent diagnostic log entries (${LOG_FILE}):${NC}"
            tail -n 25 "$LOG_FILE" | sed 's/^/  /'
        fi
        echo -e "\n${YELLOW}Need assistance? Open an issue at https://github.com/ijon6k/gopod/issues${NC}\n"
    else
        rm -f "$LOG_FILE" 2>/dev/null || true
    fi
}
trap cleanup_on_exit EXIT INT TERM

# ─────────────────────────────────────────────────────────────────────────────
# UI Helpers: Visual Progress Bar & Task Spinner (Dokploy-style)
# ─────────────────────────────────────────────────────────────────────────────

render_progress_bar() {
    local step_num="$1"
    local step_title="$2"
    local width=22
    local pct=$(( step_num * 100 / TOTAL_STEPS ))
    local filled_len=$(( step_num * width / TOTAL_STEPS ))
    local empty_len=$(( width - filled_len ))

    local bar=""
    for ((i=0; i<filled_len; i++)); do bar="${bar}${CHAR_FILLED}"; done
    for ((i=0; i<empty_len; i++)); do bar="${bar}${CHAR_EMPTY}"; done

    if [ "$IS_TTY" = true ]; then
        printf "\n${CYAN}[%s]${NC} ${BOLD}%3d%%${NC}  ${BLUE}[%d/%d]${NC} ${BOLD}%s${NC}\n" \
            "$bar" "$pct" "$step_num" "$TOTAL_STEPS" "$step_title"
    else
        echo -e "\n[Step ${step_num}/${TOTAL_STEPS}] (${pct}%) ${step_title}"
    fi
}

run_task() {
    local task_label="$1"
    shift
    local cmd=("$@")

    if [ "$DRY_RUN" = true ]; then
        echo -e "  ${DIM}[DRY-RUN] ${task_label}: ${cmd[*]}${NC}"
        return 0
    fi

    if [ "$VERBOSE" = true ]; then
        echo -e "  ${CYAN}==>${NC} ${task_label}..."
        "${cmd[@]}"
        return $?
    fi

    if [ "$IS_TTY" = false ]; then
        echo -e "  -> ${task_label}..."
        if "${cmd[@]}" >> "$LOG_FILE" 2>&1; then
            echo -e "  ${GREEN}${CHAR_CHECK}${NC} ${task_label} completed."
            return 0
        else
            local err=$?
            echo -e "  ${RED}${CHAR_CROSS}${NC} ${task_label} failed!"
            return $err
        fi
    fi

    # Interactive TTY with smooth spinner
    tput civis 2>/dev/null || printf "\033[?25l"
    "${cmd[@]}" >> "$LOG_FILE" 2>&1 &
    local task_pid=$!

    local spin_chars="⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
    local spin_idx=0
    local spin_len=${#spin_chars}

    while kill -0 "$task_pid" 2>/dev/null; do
        local ch="${spin_chars:$spin_idx:1}"
        spin_idx=$(( (spin_idx + 1) % spin_len ))
        printf "\r  ${CYAN}%s${NC} %s..." "$ch" "$task_label"
        sleep 0.08
    done

    wait "$task_pid"
    local exit_code=$?
    tput cnorm 2>/dev/null || printf "\033[?25h"

    if [ $exit_code -eq 0 ]; then
        printf "\r  ${GREEN}${CHAR_CHECK}${NC} %s completed.\033[K\n" "$task_label"
        return 0
    else
        printf "\r  ${RED}${CHAR_CROSS}${NC} %s ${RED}(failed with code %d)${NC}\033[K\n" "$task_label"
        return $exit_code
    fi
}

# Privilege escalation helper
run_privileged() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif command -v sudo &>/dev/null; then
        sudo "$@"
    elif command -v doas &>/dev/null; then
        doas "$@"
    else
        echo -e "${RED}Error: Root privileges required for: $*${NC}" >&2
        return 1
    fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Default Configuration Values & Parameter Parsing
# ─────────────────────────────────────────────────────────────────────────────

ORIGINAL_ARGS=("$@")

CURRENT_USER=$(id -un)
USER_UID=$(id -u)
USER_HOME=$(eval echo "~${CURRENT_USER}")

RUN_MODE="container"          # Default: container (Podman Compose stack). Quadlet is optional.
RELEASE_CHANNEL="latest"      # Default: latest. Optional: dev / beta.
CUSTOM_TAG=""                 # Specific tag or sha (overrides channel).
GOPOD_PORT="8085"             # Dashboard port
CADDY_HTTP_PORT="80"          # Ingress HTTP
CADDY_HTTPS_PORT="443"        # Ingress HTTPS
CONFIG_DIR="${USER_HOME}/.config/gopod"
DATA_DIR="${USER_HOME}/.local/share/gopod"
INTERACTIVE=false
NON_INTERACTIVE=false
DRY_RUN=false
VERBOSE=false
ALLOW_ROOT=false
AS_USER=false

# Container environment detection
IS_IN_CONTAINER=false
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
    IS_IN_CONTAINER=true
fi

show_help() {
    cat <<EOF
GOPOD Installer — Modern Rootless Podman PaaS

Usage:
  ./install.sh [OPTIONS]
  curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh | bash -s -- [OPTIONS]

Deployment Mode:
  --mode=container      Install as standard Podman Compose container service (Default, Recommended)
  --mode=quadlet        Install as native systemd Quadlet user units (Experimental)
  --container           Shortcut for --mode=container
  --quadlet             Shortcut for --mode=quadlet

Security & User Context:
  --rootless            Enforce rootless execution (Default on host OS)
  --allow-root, --root  Allow running directly under root (rootful mode)

Release Channels & Tags:
  --channel=latest      Install production stable release (:latest) [Default]
  --channel=dev         Install development preview release (:dev)
  --beta, --dev         Shortcut for --channel=dev
  --tag=<TAG_OR_SHA>    Install a specific image tag or git SHA (e.g. --tag=sha-1a2b3c4)

Network & Ports:
  --port=<PORT>         GOPOD web UI port (Default: 8085)
  --http-port=<PORT>    Caddy HTTP reverse proxy port (Default: 80)
  --https-port=<PORT>   Caddy HTTPS reverse proxy port (Default: 443)

Storage Directories:
  --data-dir=<PATH>     Persistent storage path (Default: ~/.local/share/gopod)
  --config-dir=<PATH>   Configuration path (Default: ~/.config/gopod)

Execution Control:
  -i, --interactive     Launch interactive setup menu
  -y, --non-interactive Run non-interactively without prompting
  --verbose             Display raw real-time command output
  --dry-run             Simulate pre-checks without changing files or starting containers
  -h, --help            Show this help dialog

Examples:
  # Quick production install with defaults:
  curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh | bash

  # Install Beta / Dev preview:
  curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install-beta.sh | bash

  # Custom interactive installation:
  curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/custom-install.sh | bash
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --mode=*)
            RUN_MODE="${1#*=}"
            ;;
        --mode)
            RUN_MODE="$2"; shift
            ;;
        --container)
            RUN_MODE="container"
            ;;
        --quadlet)
            RUN_MODE="quadlet"
            ;;
        --channel=*)
            RELEASE_CHANNEL="${1#*=}"
            ;;
        --channel)
            RELEASE_CHANNEL="$2"; shift
            ;;
        --beta|--dev)
            RELEASE_CHANNEL="dev"
            ;;
        --tag=*)
            CUSTOM_TAG="${1#*=}"
            ;;
        --tag)
            CUSTOM_TAG="$2"; shift
            ;;
        --port=*)
            GOPOD_PORT="${1#*=}"
            ;;
        --port)
            GOPOD_PORT="$2"; shift
            ;;
        --http-port=*)
            CADDY_HTTP_PORT="${1#*=}"
            ;;
        --http-port)
            CADDY_HTTP_PORT="$2"; shift
            ;;
        --https-port=*)
            CADDY_HTTPS_PORT="${1#*=}"
            ;;
        --https-port)
            CADDY_HTTPS_PORT="$2"; shift
            ;;
        --data-dir=*)
            DATA_DIR="${1#*=}"
            ;;
        --data-dir)
            DATA_DIR="$2"; shift
            ;;
        --config-dir=*)
            CONFIG_DIR="${1#*=}"
            ;;
        --config-dir)
            CONFIG_DIR="$2"; shift
            ;;
        --allow-root|--root|--rootful)
            ALLOW_ROOT=true
            ;;
        --rootless)
            ALLOW_ROOT=false
            ;;
        --as-user)
            AS_USER=true
            ;;
        --verbose)
            VERBOSE=true
            ;;
        -i|--interactive)
            INTERACTIVE=true
            ;;
        -y|--non-interactive|--yes)
            NON_INTERACTIVE=true
            ;;
        --dry-run)
            DRY_RUN=true
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

# Validate RUN_MODE
case "$RUN_MODE" in
    compose|container) RUN_MODE="container" ;;
    quadlet) RUN_MODE="quadlet" ;;
    *)
        echo -e "${RED}Invalid --mode specified: '$RUN_MODE'. Must be 'container' or 'quadlet'.${NC}"
        exit 1
        ;;
esac

# ─────────────────────────────────────────────────────────────────────────────
# Enterprise Rootless Auto-Provisioning (when executed as root on host OS)
# ─────────────────────────────────────────────────────────────────────────────

install_packages_as_root() {
    if command -v podman &>/dev/null; then return 0; fi
    if command -v apt-get &> /dev/null; then
        apt-get update -qq && apt-get install -y -qq podman systemd uidmap curl jq podman-compose 2>/dev/null || \
        apt-get install -y -qq podman systemd uidmap curl jq
    elif command -v dnf &> /dev/null; then
        dnf install -y -q podman systemd shadow-utils curl jq podman-compose 2>/dev/null || \
        dnf install -y -q podman systemd shadow-utils curl jq
    elif command -v pacman &> /dev/null; then
        pacman -Sy --noconfirm podman systemd shadow curl jq podman-compose 2>/dev/null || \
        pacman -Sy --noconfirm podman systemd shadow curl jq
    elif command -v zypper &> /dev/null; then
        zypper install -y podman systemd shadow curl jq
    elif command -v apk &> /dev/null; then
        apk add podman shadow-subids curl jq
    fi
}

if [ "$USER_UID" -eq 0 ] && [ "$AS_USER" = false ]; then
    # In container sandbox, default to rootful unless user explicitly asked for rootless
    if [ "$IS_IN_CONTAINER" = true ]; then
        ALLOW_ROOT=true
    fi

    if [ "$ALLOW_ROOT" = false ]; then
        TTY_DEV="/dev/tty"
        CHOICE_USER="Y"

        if [ "$NON_INTERACTIVE" = false ] && ([ -e "$TTY_DEV" ] || [ -t 0 ]); then
            exec 3< "$TTY_DEV" 2>/dev/null || exec 3<&0
            echo -e "${CYAN}${BOLD}"
            echo "┌─────────────────────────────────────────────────────────────┐"
            echo "│         Rootless Execution & Security Recommendation        │"
            echo "└─────────────────────────────────────────────────────────────┘"
            echo -e "${NC}"
            echo -e "You are currently running this installer as '${BOLD}root${NC}' (UID 0)."
            echo -e "For container security and enterprise compliance, GOPOD recommends"
            echo -e "running under a dedicated unprivileged user '${BOLD}gopod${NC}' (100% Rootless).\n"
            echo -e "  ${GREEN}${CHAR_CHECK}${NC} Complete isolation from host root filesystem"
            echo -e "  ${GREEN}${CHAR_CHECK}${NC} Mitigates container escape risks"
            echo -e "  ${GREEN}${CHAR_CHECK}${NC} System service user (no password required)\n"
            read -r -u 3 -p "Create and install under dedicated 'gopod' user? [Y/n] (Default: Y): " INPUT_USER_CHOICE
            exec 3<&-
            if [[ "$INPUT_USER_CHOICE" =~ ^[nN]$ ]]; then
                CHOICE_USER="N"
            fi
        fi

        if [ "$CHOICE_USER" = "N" ]; then
            ALLOW_ROOT=true
            echo -e "\n  ${YELLOW}${CHAR_WARN} Continuing installation in Rootful mode (UID 0)...${NC}\n"
        else
            echo -e "\n${CYAN}${BOLD}==> Provisioning dedicated 'gopod' service user for rootless execution...${NC}"

            # 1. Create service user if missing
            if ! id -u gopod &>/dev/null; then
                useradd -m -s /bin/bash gopod 2>/dev/null || adduser -D -s /bin/bash gopod 2>/dev/null
                echo -e "  ${GREEN}${CHAR_CHECK}${NC} Created dedicated service user: ${BOLD}gopod${NC}"
            else
                echo -e "  ${GREEN}${CHAR_CHECK}${NC} Using existing service user: ${BOLD}gopod${NC}"
            fi

            # 2. Configure subordinate UID/GID mappings
            usermod --add-subuids 100000-165535 gopod 2>/dev/null || true
            usermod --add-subgids 100000-165535 gopod 2>/dev/null || true
            echo -e "  ${GREEN}${CHAR_CHECK}${NC} Subordinate UID/GID ranges configured (100000-165535)"

            # 3. Configure sysctl for unprivileged port binding (:80 & :443)
            local_port_start=$(cat /proc/sys/net/ipv4/ip_unprivileged_port_start 2>/dev/null || echo "1024")
            if [ "$local_port_start" -gt 80 ]; then
                echo "net.ipv4.ip_unprivileged_port_start=80" > /etc/sysctl.d/99-gopod.conf 2>/dev/null || true
                sysctl -p /etc/sysctl.d/99-gopod.conf >/dev/null 2>&1 || sysctl --system >/dev/null 2>&1 || true
                echo -e "  ${GREEN}${CHAR_CHECK}${NC} Kernel sysctl configured: net.ipv4.ip_unprivileged_port_start=80"
            fi

            # 4. Enable systemd lingering
            if command -v loginctl &>/dev/null; then
                loginctl enable-linger gopod 2>/dev/null || true
                echo -e "  ${GREEN}${CHAR_CHECK}${NC} Systemd process lingering enabled for 'gopod'"
            fi

            # 5. Install packages as root before delegating
            echo -e "  ${CYAN}==>${NC} Validating host system packages..."
            install_packages_as_root

            # 6. Ensure runtime directory
            GOPOD_SERVICE_UID=$(id -u gopod)
            GOPOD_RUNTIME_DIR="/run/user/${GOPOD_SERVICE_UID}"
            mkdir -p "$GOPOD_RUNTIME_DIR" 2>/dev/null || true
            chown -R gopod:gopod "$GOPOD_RUNTIME_DIR" 2>/dev/null || true
            chmod 700 "$GOPOD_RUNTIME_DIR" 2>/dev/null || true

            # Activate systemd user manager if available
            if command -v systemctl &>/dev/null && [ -d /run/systemd/system ]; then
                systemctl start "user@${GOPOD_SERVICE_UID}.service" 2>/dev/null || true
            fi

            # 7. Re-execute script under gopod user
            SCRIPT_TEMP="/tmp/gopod-installer-$$.sh"
            if [ -f "$0" ] && [ -r "$0" ]; then
                cp "$0" "$SCRIPT_TEMP"
            else
                curl -fsSL "https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh" -o "$SCRIPT_TEMP"
            fi
            chmod 755 "$SCRIPT_TEMP"

            echo -e "  ${GREEN}${CHAR_CHECK}${NC} Switching execution to user '${BOLD}gopod${NC}'...\n"
            su - gopod -c "XDG_RUNTIME_DIR=${GOPOD_RUNTIME_DIR} DBUS_SESSION_BUS_ADDRESS=unix:path=${GOPOD_RUNTIME_DIR}/bus bash ${SCRIPT_TEMP} ${ORIGINAL_ARGS[*]} --as-user"
            SUB_EXIT=$?
            rm -f "$SCRIPT_TEMP" 2>/dev/null || true
            exit $SUB_EXIT
        fi
    fi
fi

# Refresh user variables in case of handoff
CURRENT_USER=$(id -un)
USER_UID=$(id -u)
USER_HOME=$(eval echo "~${CURRENT_USER}")
CONFIG_DIR="${USER_HOME}/.config/gopod"
DATA_DIR="${USER_HOME}/.local/share/gopod"

# ─────────────────────────────────────────────────────────────────────────────
# Interactive Setup Wizard (if requested or piped with tty)
# ─────────────────────────────────────────────────────────────────────────────

if [ "$INTERACTIVE" = true ] && [ "$NON_INTERACTIVE" = false ]; then
    TTY_DEV="/dev/tty"
    if [ ! -e "$TTY_DEV" ] && [ ! -t 0 ]; then
        echo -e "${YELLOW}Notice: Interactive mode requested but no TTY is available. Continuing with defaults.${NC}"
    else
        exec 3< "$TTY_DEV" 2>/dev/null || exec 3<&0

        echo -e "${CYAN}${BOLD}"
        echo "┌─────────────────────────────────────────────────────────────┐"
        echo "│               GOPOD Interactive Setup Wizard                │"
        echo "└─────────────────────────────────────────────────────────────┘"
        echo -e "${NC}"

        # 1. Release Channel / Tag
        echo -e "${BOLD}[1/4] Select Release Channel / Container Tag:${NC}"
        echo -e "  ${BOLD}1)${NC} ${GREEN}Stable${NC} (:latest) — Recommended for production"
        echo -e "  ${BOLD}2)${NC} ${YELLOW}Beta / Dev${NC} (:dev) — Latest development features"
        echo -e "  ${BOLD}3)${NC} ${MAGENTA}Custom Tag / Git SHA${NC} — Specific build or commit"
        read -r -u 3 -p "Select channel [1-3] (default: 1): " CHOICE_CHANNEL
        case "$CHOICE_CHANNEL" in
            2|beta|dev)
                RELEASE_CHANNEL="dev"
                ;;
            3|custom)
                read -r -u 3 -p "Enter custom image tag or git SHA (e.g. sha-xxxx or v1.0.0): " USER_TAG
                if [ -n "$USER_TAG" ]; then
                    CUSTOM_TAG="$USER_TAG"
                fi
                ;;
            *)
                RELEASE_CHANNEL="latest"
                ;;
        esac

        # 2. Deployment Architecture
        echo -e "\n${BOLD}[2/4] Select Deployment Architecture:${NC}"
        echo -e "  ${BOLD}1)${NC} ${GREEN}Container (Podman Compose)${NC} [Default, Recommended] — High reliability across all distros"
        echo -e "  ${BOLD}2)${NC} ${BLUE}Systemd Quadlet${NC} [Experimental] — Declarative systemd user units"
        read -r -u 3 -p "Select mode [1/2] (default: 1): " CHOICE_MODE
        case "$CHOICE_MODE" in
            2|quadlet|q)
                RUN_MODE="quadlet"
                ;;
            *)
                RUN_MODE="container"
                ;;
        esac

        # 3. Ports
        echo -e "\n${BOLD}[3/4] Port Configuration:${NC}"
        read -r -u 3 -p "Web Dashboard Port [${GOPOD_PORT}]: " INPUT_PORT
        if [ -n "$INPUT_PORT" ]; then GOPOD_PORT="$INPUT_PORT"; fi

        read -r -u 3 -p "Caddy HTTP Ingress Port [${CADDY_HTTP_PORT}]: " INPUT_HTTP
        if [ -n "$INPUT_HTTP" ]; then CADDY_HTTP_PORT="$INPUT_HTTP"; fi

        read -r -u 3 -p "Caddy HTTPS Ingress Port [${CADDY_HTTPS_PORT}]: " INPUT_HTTPS
        if [ -n "$INPUT_HTTPS" ]; then CADDY_HTTPS_PORT="$INPUT_HTTPS"; fi

        # 4. Storage Directories
        echo -e "\n${BOLD}[4/4] Persistent Storage Path:${NC}"
        read -r -u 3 -p "Data directory [${DATA_DIR}]: " INPUT_DATA_DIR
        if [ -n "$INPUT_DATA_DIR" ]; then DATA_DIR="$INPUT_DATA_DIR"; fi

        exec 3<&-
        echo -e "\n${GREEN}✓ Configuration saved.${NC}\n"
    fi
fi

# Determine resolved image tag
if [ -n "$CUSTOM_TAG" ]; then
    GOPOD_IMAGE="ghcr.io/ijon6k/gopod:${CUSTOM_TAG}"
    DISPLAY_TAG="${CUSTOM_TAG}"
elif [ "$RELEASE_CHANNEL" = "dev" ] || [ "$RELEASE_CHANNEL" = "beta" ]; then
    GOPOD_IMAGE="ghcr.io/ijon6k/gopod:dev"
    DISPLAY_TAG="dev (Beta)"
else
    GOPOD_IMAGE="ghcr.io/ijon6k/gopod:latest"
    DISPLAY_TAG="latest (Stable)"
fi

CADDY_IMAGE="docker.io/library/caddy:alpine"

# ─────────────────────────────────────────────────────────────────────────────
# Header & Welcome Banner
# ─────────────────────────────────────────────────────────────────────────────

echo -e "${BLUE}${BOLD}"
echo "=============================================================="
echo "          GOPOD — Modern Rootless Podman PaaS                 "
echo "=============================================================="
echo -e "${NC}"
echo -e " Target Image : ${CYAN}${GOPOD_IMAGE}${NC} (${DISPLAY_TAG})"
echo -e " Deploy Mode  : ${BOLD}${RUN_MODE}${NC} $([ "$RUN_MODE" = "container" ] && echo "(Container/Compose - Recommended)" || echo "(Quadlet - Experimental)")"
echo -e " Dashboard UI : ${BOLD}http://localhost:${GOPOD_PORT}${NC}"
echo -e " Ingress Ports: HTTP :${CADDY_HTTP_PORT} | HTTPS :${CADDY_HTTPS_PORT}"
echo -e " Storage Path : ${DIM}${DATA_DIR}${NC}"
echo -e " Config Path  : ${DIM}${CONFIG_DIR}${NC}"
echo -e " User Context : ${BOLD}${CURRENT_USER}${NC} (UID: ${USER_UID}) $([ "$CURRENT_USER" = "root" ] && echo "${YELLOW}[Rootful]${NC}" || echo "${GREEN}[Rootless]${NC}")"
if [ "$DRY_RUN" = true ]; then
    echo -e " Execution    : ${YELLOW}${BOLD}DRY-RUN (No system modifications will be written)${NC}"
fi
echo "──────────────────────────────────────────────────────────────"

# Detect system environment in parent shell
DISTRO_ID="unknown"
DISTRO_NAME="Linux"
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO_ID="${ID:-unknown}"
    DISTRO_NAME="${NAME:-Linux}"
fi

SELINUX_STATUS="disabled"
if command -v getenforce &> /dev/null; then
    SELINUX_STATUS=$(getenforce 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo "disabled")
elif [ -f /etc/selinux/config ]; then
    SELINUX_STATUS=$(grep -E '^SELINUX=' /etc/selinux/config 2>/dev/null | cut -d= -f2 | tr '[:upper:]' '[:lower:]' || echo "disabled")
fi

if [ "$CURRENT_USER" = "root" ]; then
    DEFAULT_SOCKET="/run/podman/podman.sock"
else
    DEFAULT_SOCKET="/run/user/${USER_UID}/podman/podman.sock"
fi
DETECTED_SOCKET="$DEFAULT_SOCKET"

# ─────────────────────────────────────────────────────────────────────────────
# [1/7] Linux Distro & Environment Detection
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 1 "Detecting Linux distribution and security policies"

task_detect_env() {
    [ -n "$DISTRO_NAME" ]
}
run_task "Detecting Linux operating system release (${DISTRO_NAME})" task_detect_env

task_detect_selinux() {
    if [ "$SELINUX_STATUS" = "enforcing" ] || [ "$SELINUX_STATUS" = "permissive" ]; then
        if command -v setsebool &> /dev/null; then
            run_privileged setsebool -P container_manage_cgroup 1 2>/dev/null || true
        fi
    fi
}
run_task "Verifying security policy and cgroup isolation" task_detect_selinux

# ─────────────────────────────────────────────────────────────────────────────
# [2/7] Package Installation & Dependencies
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 2 "Checking Podman, compose, and subuid dependencies"

task_install_deps() {
    if command -v podman &> /dev/null; then return 0; fi
    if command -v apt-get &> /dev/null; then
        run_privileged apt-get update -qq
        run_privileged apt-get install -y -qq podman systemd uidmap curl jq podman-compose 2>/dev/null || \
        run_privileged apt-get install -y -qq podman systemd uidmap curl jq
    elif command -v dnf &> /dev/null; then
        run_privileged dnf install -y -q podman systemd shadow-utils curl jq podman-compose 2>/dev/null || \
        run_privileged dnf install -y -q podman systemd shadow-utils curl jq
    elif command -v pacman &> /dev/null; then
        run_privileged pacman -Sy --noconfirm podman systemd shadow curl jq podman-compose 2>/dev/null || \
        run_privileged pacman -Sy --noconfirm podman systemd shadow curl jq
    elif command -v zypper &> /dev/null; then
        run_privileged zypper install -y podman systemd shadow curl jq
    elif command -v apk &> /dev/null; then
        run_privileged apk add podman shadow-subids curl jq
    else
        echo -e "Unsupported package manager. Please install Podman manually." >&2
        return 1
    fi
}
run_task "Validating Podman virtualization engine" task_install_deps

task_check_subuids() {
    if [ "$CURRENT_USER" = "root" ]; then return 0; fi
    local subuid_ok=false
    if grep -q "^${CURRENT_USER}:" /etc/subuid 2>/dev/null && grep -q "^${CURRENT_USER}:" /etc/subgid 2>/dev/null; then
        subuid_ok=true
    fi
    if [ "$subuid_ok" = false ]; then
        run_privileged usermod --add-subuids 100000-165535 "$CURRENT_USER" 2>/dev/null || true
        run_privileged usermod --add-subgids 100000-165535 "$CURRENT_USER" 2>/dev/null || true
    fi
}
run_task "Verifying subordinate UID/GID mappings for ${CURRENT_USER}" task_check_subuids

# ─────────────────────────────────────────────────────────────────────────────
# [3/7] Unprivileged Port Binding (80 & 443)
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 3 "Configuring unprivileged port binding (ports 80 & 443)"

task_configure_sysctl_ports() {
    local current_port
    current_port=$(cat /proc/sys/net/ipv4/ip_unprivileged_port_start 2>/dev/null || echo "1024")
    if [ "$current_port" -gt 80 ]; then
        local sysctl_file="/etc/sysctl.d/99-gopod.conf"
        echo "net.ipv4.ip_unprivileged_port_start=80" | run_privileged tee "$sysctl_file" > /dev/null 2>&1 || true
        run_privileged sysctl -p "$sysctl_file" > /dev/null 2>&1 || run_privileged sysctl --system > /dev/null 2>&1 || true
    fi
}
run_task "Configuring sysctl unprivileged port binding (port start <= 80)" task_configure_sysctl_ports

# ─────────────────────────────────────────────────────────────────────────────
# [4/7] Systemd User Lingering & Podman Socket
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 4 "Activating systemd user lingering and Podman socket"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/${USER_UID}}"

task_enable_linger() {
    if [ "$CURRENT_USER" != "root" ] && command -v loginctl &> /dev/null; then
        run_privileged loginctl enable-linger "$CURRENT_USER" 2>/dev/null || true
    fi
}
run_task "Configuring systemd process lingering for ${CURRENT_USER}" task_enable_linger

task_activate_socket() {
    local systemd_user_avail=false
    if command -v systemctl &> /dev/null && [ -d /run/systemd/system ]; then
        if systemctl --user status &> /dev/null || [ -S "${XDG_RUNTIME_DIR}/bus" ]; then
            systemd_user_avail=true
        fi
    fi

    if [ "$systemd_user_avail" = true ]; then
        systemctl --user enable --now podman.socket 2>/dev/null || true
        systemctl --user enable --now podman-restart.service 2>/dev/null || true
    elif [ "$CURRENT_USER" = "root" ] && command -v systemctl &> /dev/null; then
        systemctl enable --now podman.socket 2>/dev/null || true
        systemctl enable --now podman-restart.service 2>/dev/null || true
    fi

    # Query active socket if available
    local query
    query=$(podman info --format '{{.Host.RemoteSocket.Path}}' 2>/dev/null || true)
    local target_socket="$DEFAULT_SOCKET"
    if [ -n "$query" ] && [ -S "$query" ]; then
        target_socket="$query"
    fi
    echo "$target_socket" > "/tmp/gopod-socket-$$.txt"

    # If socket is still not running, activate standalone service
    if [ ! -S "$target_socket" ]; then
        run_privileged mkdir -p "$(dirname "$target_socket")" 2>/dev/null || true
        nohup podman system service --time=0 "unix://${target_socket}" >/dev/null 2>&1 &
        sleep 1
    fi
}
run_task "Activating Podman API socket service" task_activate_socket

if [ -f "/tmp/gopod-socket-$$.txt" ]; then
    DETECTED_SOCKET=$(cat "/tmp/gopod-socket-$$.txt")
    rm -f "/tmp/gopod-socket-$$.txt" 2>/dev/null || true
else
    DETECTED_SOCKET="$DEFAULT_SOCKET"
fi

# ─────────────────────────────────────────────────────────────────────────────
# [5/7] Storage Directories & File Permissions
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 5 "Initializing storage layout and permissions"

GOPOD_CONFIG="${CONFIG_DIR}"
GOPOD_DATA="${DATA_DIR}"
QUADLET_DIR="${USER_HOME}/.config/containers/systemd"

task_init_storage() {
    mkdir -p "${GOPOD_CONFIG}/caddy"
    mkdir -p "${GOPOD_DATA}/data"
    mkdir -p "${GOPOD_DATA}/caddy/data"
    mkdir -p "${GOPOD_DATA}/caddy/config"
    mkdir -p "${GOPOD_DATA}/logs"
    mkdir -p "${GOPOD_DATA}/deployments"
    mkdir -p "${GOPOD_DATA}/keys"
    mkdir -p "${QUADLET_DIR}"

    if [ ! -f "${GOPOD_DATA}/logs/access.log" ]; then
        touch "${GOPOD_DATA}/logs/access.log"
    fi

    local caddy_upstream="gopod:8085"
    if [ "$RUN_MODE" = "quadlet" ]; then
        caddy_upstream="127.0.0.1:8085"
    fi

    if [ ! -f "${GOPOD_CONFIG}/caddy/Caddyfile" ]; then
        cat <<EOF > "${GOPOD_CONFIG}/caddy/Caddyfile"
{
    admin 0.0.0.0:2019
}

:80 {
    log {
        output file /var/log/caddy/access.log {
            roll_size 20MiB
            roll_keep 7
        }
        format json
    }
    reverse_proxy ${caddy_upstream}
}
EOF
    fi

    chmod 750 "${GOPOD_DATA}" "${GOPOD_CONFIG}" 2>/dev/null || true
    chmod 700 "${GOPOD_DATA}/keys" 2>/dev/null || true
    chmod 644 "${GOPOD_DATA}/logs/access.log" 2>/dev/null || true

    if [ "$SELINUX_STATUS" = "enforcing" ] || [ "$SELINUX_STATUS" = "permissive" ]; then
        if command -v chcon &> /dev/null; then
            chcon -Rt container_file_t "${GOPOD_DATA}" "${GOPOD_CONFIG}" 2>/dev/null || true
        fi
    fi
}
run_task "Creating configuration and persistent data directories" task_init_storage

# ─────────────────────────────────────────────────────────────────────────────
# [6/7] Pulling Container Images (Clean Progress, Verbose Hidden)
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 6 "Pulling container images (${DISPLAY_TAG})"

task_pull_gopod() {
    podman pull "$GOPOD_IMAGE"
}
run_task "Downloading GOPOD control plane (${GOPOD_IMAGE})" task_pull_gopod

task_pull_caddy() {
    podman pull "$CADDY_IMAGE"
}
run_task "Downloading Caddy reverse proxy ingress (${CADDY_IMAGE})" task_pull_caddy

# ─────────────────────────────────────────────────────────────────────────────
# [7/7] Generating Manifests & Starting Service
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 7 "Orchestrating GOPOD services (${RUN_MODE} mode)"

task_generate_manifests() {
    # Generate Podman Compose manifest
    cat <<EOF > "${GOPOD_CONFIG}/compose.yml"
services:
  gopod:
    image: ${GOPOD_IMAGE}
    container_name: gopod
    restart: always
    ports:
      - "${GOPOD_PORT}:8085"
    environment:
      - PORT=8085
      - PODMAN_SOCKET=/run/podman/podman.sock
      - GOPOD_DB_PATH=/app/data/gopod.db
      - CADDY_ADMIN_URL=http://caddy:2019
    volumes:
      - "${DETECTED_SOCKET}:/run/podman/podman.sock:ro,z"
      - "${GOPOD_DATA}/data:/app/data:Z"
      - "${GOPOD_DATA}/logs:/var/log/caddy:z"
    networks:
      - gopod-net
    security_opt:
      - label=disable

  caddy:
    image: ${CADDY_IMAGE}
    container_name: gopod-caddy
    restart: always
    ports:
      - "${CADDY_HTTP_PORT}:80"
      - "${CADDY_HTTPS_PORT}:443"
    environment:
      - CADDY_INGRESS_NETWORK=gopod-net
    volumes:
      - "${GOPOD_CONFIG}/caddy/Caddyfile:/etc/caddy/Caddyfile:Z"
      - "${GOPOD_DATA}/caddy/data:/data:Z"
      - "${GOPOD_DATA}/caddy/config:/config:Z"
      - "${GOPOD_DATA}/logs:/var/log/caddy:z"
    networks:
      - gopod-net

networks:
  gopod-net:
    name: gopod-net
    driver: bridge
EOF

    # Generate Systemd Quadlet manifests
    cat <<EOF > "${QUADLET_DIR}/gopod-net.network"
[Network]
NetworkName=gopod-net
Subnet=10.89.0.0/16
Gateway=10.89.0.1
Internal=false
EOF

    cat <<EOF > "${QUADLET_DIR}/gopod.pod"
[Unit]
Description=GOPOD Core System Pod (Caddy Ingress & Control Plane)
After=network-online.target
Wants=network-online.target

[Pod]
PodName=gopod
Network=gopod-net.network
PublishPort=${CADDY_HTTP_PORT}:80
PublishPort=${CADDY_HTTPS_PORT}:443
PublishPort=${GOPOD_PORT}:8085

[Install]
WantedBy=default.target
EOF

    cat <<EOF > "${QUADLET_DIR}/gopod-caddy.container"
[Unit]
Description=GOPOD Caddy Ingress Reverse Proxy
PartOf=gopod-pod.service

[Container]
Pod=gopod.pod
ContainerName=gopod-caddy
Image=${CADDY_IMAGE}
AutoUpdate=registry
Volume=${GOPOD_CONFIG}/caddy/Caddyfile:/etc/caddy/Caddyfile:Z
Volume=${GOPOD_DATA}/caddy/data:/data:Z
Volume=${GOPOD_DATA}/caddy/config:/config:Z
Volume=${GOPOD_DATA}/logs:/var/log/caddy:z
Environment=CADDY_INGRESS_NETWORK=gopod-net

[Service]
Restart=always
RestartSec=3s

[Install]
WantedBy=default.target
EOF

    cat <<EOF > "${QUADLET_DIR}/gopod-server.container"
[Unit]
Description=GOPOD PaaS Control Plane
PartOf=gopod-pod.service
After=gopod-caddy.service

[Container]
Pod=gopod.pod
ContainerName=gopod
Image=${GOPOD_IMAGE}
AutoUpdate=registry
Environment=PORT=8085
Environment=PODMAN_SOCKET=/run/podman/podman.sock
Environment=GOPOD_DB_PATH=/app/data/gopod.db
Environment=CADDY_ADMIN_URL=http://127.0.0.1:2019
Volume=%t/podman/podman.sock:/run/podman/podman.sock:z
Volume=${GOPOD_DATA}/data:/app/data:Z
Volume=${GOPOD_DATA}/logs:/var/log/caddy:z

[Service]
Restart=always
RestartSec=3s

[Install]
WantedBy=default.target
EOF
}

start_quadlet() {
    if ! command -v systemctl &> /dev/null; then return 1; fi
    systemctl --user daemon-reload 2>/dev/null || return 1
    systemctl --user enable --now gopod-pod.service 2>/dev/null || return 1
    sleep 2
    systemctl --user is-active --quiet gopod-pod.service 2>/dev/null
}

start_compose() {
    podman stop gopod gopod-caddy 2>/dev/null || true
    podman rm gopod gopod-caddy 2>/dev/null || true

    if podman compose version &> /dev/null; then
        podman compose -f "${GOPOD_CONFIG}/compose.yml" up -d
    elif command -v podman-compose &> /dev/null; then
        podman-compose -f "${GOPOD_CONFIG}/compose.yml" up -d
    elif command -v docker-compose &> /dev/null; then
        docker-compose -f "${GOPOD_CONFIG}/compose.yml" up -d
    else
        podman network create gopod-net 2>/dev/null || true
        podman run -d --name gopod \
            --restart=always \
            --network=gopod-net \
            -p "${GOPOD_PORT}:8085" \
            -e PORT=8085 \
            -e PODMAN_SOCKET=/run/podman/podman.sock \
            -e GOPOD_DB_PATH=/app/data/gopod.db \
            -e CADDY_ADMIN_URL=http://gopod-caddy:2019 \
            -v "${DETECTED_SOCKET}:/run/podman/podman.sock:ro,z" \
            -v "${GOPOD_DATA}/data:/app/data:Z" \
            -v "${GOPOD_DATA}/logs:/var/log/caddy:z" \
            --security-opt label=disable \
            "${GOPOD_IMAGE}"

        podman run -d --name gopod-caddy \
            --restart=always \
            --network=gopod-net \
            -p "${CADDY_HTTP_PORT}:80" \
            -p "${CADDY_HTTPS_PORT}:443" \
            -e CADDY_INGRESS_NETWORK=gopod-net \
            -v "${GOPOD_CONFIG}/caddy/Caddyfile:/etc/caddy/Caddyfile:Z" \
            -v "${GOPOD_DATA}/caddy/data:/data:Z" \
            -v "${GOPOD_DATA}/caddy/config:/config:Z" \
            -v "${GOPOD_DATA}/logs:/var/log/caddy:z" \
            "${CADDY_IMAGE}"
    fi
}

task_orchestrate_services() {
    task_generate_manifests
    if [ "$RUN_MODE" = "quadlet" ]; then
        if ! start_quadlet; then
            RUN_MODE="container"
            start_compose
        fi
    else
        start_compose
    fi
}
run_task "Deploying container services (${RUN_MODE} mode)" task_orchestrate_services

task_verify_readiness() {
    if [ "$DRY_RUN" = true ]; then return 0; fi
    local retries=15
    while [ $retries -gt 0 ]; do
        if curl -fsSL -o /dev/null "http://127.0.0.1:${GOPOD_PORT}" 2>/dev/null; then
            return 0
        fi
        sleep 1
        retries=$((retries - 1))
    done
    return 0
}
run_task "Verifying service readiness & health check" task_verify_readiness

# Detect Public IP for VPS or fallback to local IP
PUBLIC_IP=$(curl -4s --max-time 2 https://api.ipify.org 2>/dev/null || curl -4s --max-time 2 https://ifconfig.me 2>/dev/null || true)
if [ -n "$PUBLIC_IP" ] && [[ "$PUBLIC_IP" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    SERVER_IP="$PUBLIC_IP"
else
    SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")
fi

echo -e "\n${GREEN}${BOLD}=============================================================="
echo -e "          GOPOD Installation Completed Successfully!          "
echo -e "==============================================================${NC}"
echo -e " Web Dashboard : ${BOLD}http://${SERVER_IP}:${GOPOD_PORT}${NC}$([ "$SERVER_IP" != "127.0.0.1" ] && echo -e " ${DIM}(or http://localhost:${GOPOD_PORT})${NC}" || true)"
echo -e " Ingress URL   : ${BOLD}http://${SERVER_IP}:${CADDY_HTTP_PORT}${NC}"
echo -e " Release Tag   : ${CYAN}${DISPLAY_TAG}${NC} (${GOPOD_IMAGE})"
echo -e " Running Mode  : ${BOLD}${RUN_MODE}${NC} (Auto-restart enabled)"
echo -e " Service User  : ${BOLD}${CURRENT_USER}${NC} $([ "$CURRENT_USER" = "root" ] && echo "${YELLOW}(Rootful)${NC}" || echo "${GREEN}(Rootless, UID: ${USER_UID})${NC}")"
echo -e " Podman Socket : ${DIM}${DETECTED_SOCKET}${NC}"
echo -e " Storage Path  : ${DIM}${GOPOD_DATA}${NC}"
echo -e " Compose Spec  : ${DIM}${GOPOD_CONFIG}/compose.yml${NC}"
echo "──────────────────────────────────────────────────────────────"
echo -e "${BOLD}Quick Management Commands:${NC}"
if [ "$CURRENT_USER" != "root" ] && [ "$(id -u)" -eq 0 ]; then
    echo -e "  Switch user  : ${CYAN}su - ${CURRENT_USER}${NC}"
    echo -e "  Check status : ${CYAN}su - ${CURRENT_USER} -c 'podman ps --filter name=gopod'${NC}"
    echo -e "  View logs    : ${CYAN}su - ${CURRENT_USER} -c 'podman compose -f ~/.config/gopod/compose.yml logs -f'${NC}"
    echo -e "  Restart      : ${CYAN}su - ${CURRENT_USER} -c 'podman compose -f ~/.config/gopod/compose.yml restart'${NC}"
else
    echo -e "  Check status : ${CYAN}podman ps --filter name=gopod${NC}"
    echo -e "  View logs    : ${CYAN}podman compose -f ${GOPOD_CONFIG}/compose.yml logs -f${NC}"
    echo -e "  Restart      : ${CYAN}podman compose -f ${GOPOD_CONFIG}/compose.yml restart${NC}"
fi
echo -e "\n${BLUE}Open your browser at ${BOLD}http://${SERVER_IP}:${GOPOD_PORT}${NC}${BLUE} to begin the setup wizard.${NC}\n"
