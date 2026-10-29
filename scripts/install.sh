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
# UI Helpers: Step Header & Task Spinners
# ─────────────────────────────────────────────────────────────────────────────

render_progress_bar() {
    local step_num="$1"
    local step_title="$2"

    if [ "$IS_TTY" = true ]; then
        printf "\n${CYAN}${BOLD}[%d/%d]${NC} ${BOLD}%s${NC}\n" \
            "$step_num" "$TOTAL_STEPS" "$step_title"
    else
        echo -e "\n[Step ${step_num}/${TOTAL_STEPS}] ${step_title}"
    fi
}

run_live_task() {
    local task_label="$1"
    shift
    local cmd=("$@")

    if [ "$DRY_RUN" = true ]; then
        echo -e "  ${DIM}[DRY-RUN] ${task_label}: ${cmd[*]}${NC}"
        return 0
    fi

    echo -e "  ${CYAN}==>${NC} ${task_label}..."
    if "${cmd[@]}"; then
        echo -e "  ${GREEN}${CHAR_CHECK}${NC} ${task_label} completed.\n"
        return 0
    else
        local err=$?
        echo -e "  ${RED}${CHAR_CROSS}${NC} ${task_label} failed!"
        return $err
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
# GOPOD CLI Wrapper Generator (/usr/local/bin/gopod)
# ─────────────────────────────────────────────────────────────────────────────

create_cli_wrapper() {
    local wrapper_user="${1:-$CURRENT_USER}"
    local wrapper_uid="${2:-$USER_UID}"
    local wrapper_home="${3:-$USER_HOME}"
    local wrapper_config="${4:-${wrapper_home}/.config/gopod}"
    local wrapper_data="${5:-${wrapper_home}/.local/share/gopod}"
    local wrapper_port="${6:-$GOPOD_PORT}"
    local wrapper_caddy="${7:-$CADDY_HTTP_PORT}"

    local tmp_wrapper
    tmp_wrapper="$(mktemp /tmp/gopod-cli.XXXXXX 2>/dev/null || echo "/tmp/gopod-cli-$$")"

    cat << 'WRAPPER_HEADER' > "$tmp_wrapper"
#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GOPOD CLI Management Helper
# Unified CLI for rootless GOPOD PaaS & Caddy Ingress services.
# Automatically bridges user context, sockets, and environment variables.
# ─────────────────────────────────────────────────────────────────────────────
set -e

WRAPPER_HEADER

    cat << WRAPPER_VARS >> "$tmp_wrapper"
GOPOD_USER="${wrapper_user}"
GOPOD_UID="${wrapper_uid}"
GOPOD_HOME="${wrapper_home}"
GOPOD_CONFIG="${wrapper_config}"
GOPOD_DATA="${wrapper_data}"
DEFAULT_PORT="${wrapper_port}"
DEFAULT_CADDY_PORT="${wrapper_caddy}"
WRAPPER_VARS

    cat << 'WRAPPER_BODY' >> "$tmp_wrapper"
COMPOSE_FILE="${GOPOD_CONFIG}/compose.yml"
RUNTIME_DIR="/run/user/${GOPOD_UID:-$(id -u)}"
PODMAN_SOCK="${RUNTIME_DIR}/podman/podman.sock"
if [ "${GOPOD_UID:-1}" -eq 0 ]; then
    RUNTIME_DIR="/run"
    PODMAN_SOCK="/run/podman/podman.sock"
fi
QUADLET_DIR="$(dirname "${GOPOD_CONFIG}")/containers/systemd"
ADMIN_SOCK="${GOPOD_DATA}/caddy-admin/admin.sock"

# Determine active ports (dynamically from compose.yml if present, otherwise defaults)
ACTUAL_PORT="$DEFAULT_PORT"
ACTUAL_CADDY_PORT="$DEFAULT_CADDY_PORT"
if [ -f "$COMPOSE_FILE" ]; then
    FOUND_PORT=$(grep -oE '[0-9]+:8085' "$COMPOSE_FILE" 2>/dev/null | head -n1 | cut -d: -f1 || true)
    if [ -n "$FOUND_PORT" ]; then ACTUAL_PORT="$FOUND_PORT"; fi
    FOUND_CADDY=$(grep -oE '[0-9]+:80' "$COMPOSE_FILE" 2>/dev/null | head -n1 | cut -d: -f1 || true)
    if [ -n "$FOUND_CADDY" ]; then ACTUAL_CADDY_PORT="$FOUND_CADDY"; fi
fi
: "${ACTUAL_PORT:=$DEFAULT_PORT}"
: "${ACTUAL_CADDY_PORT:=$DEFAULT_CADDY_PORT}"

BOLD="$(printf '\033[1m')"
DIM="$(printf '\033[2m')"
GREEN="$(printf '\033[0;32m')"
YELLOW="$(printf '\033[1;33m')"
CYAN="$(printf '\033[0;36m')"
RED="$(printf '\033[0;31m')"
BLUE="$(printf '\033[0;34m')"
NC="$(printf '\033[0m')"

# Ensure rootless socket is running if systemd session exists
ensure_socket() {
    if [ "$GOPOD_UID" -ne 0 ] && [ ! -S "$PODMAN_SOCK" ]; then
        if [ "$(id -u)" -eq 0 ] && command -v systemctl &>/dev/null; then
            systemctl start "user@${GOPOD_UID}.service" 2>/dev/null || true
        fi
        run_rootless systemctl --user start podman.socket 2>/dev/null || true
    fi
}

# Run commands within service user's rootless context
run_rootless() {
    if [ "$(id -u)" -eq "${GOPOD_UID:-$(id -u)}" ]; then
        export XDG_RUNTIME_DIR="${RUNTIME_DIR}"
        export DBUS_SESSION_BUS_ADDRESS="unix:path=${RUNTIME_DIR}/bus"
        export DOCKER_HOST="unix://${PODMAN_SOCK}"
        "$@"
    elif [ "$(id -u)" -eq 0 ]; then
        local escaped_cmd=""
        for arg in "$@"; do
            escaped_cmd+="$(printf '%q ' "$arg")"
        done
        su - "$GOPOD_USER" -c "export XDG_RUNTIME_DIR='${RUNTIME_DIR}' DBUS_SESSION_BUS_ADDRESS='unix:path=${RUNTIME_DIR}/bus' DOCKER_HOST='unix://${PODMAN_SOCK}'; ${escaped_cmd}"
    elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null && [ -n "${GOPOD_USER:-}" ]; then
        sudo -n -u "$GOPOD_USER" env XDG_RUNTIME_DIR="${RUNTIME_DIR}" DBUS_SESSION_BUS_ADDRESS="unix:path=${RUNTIME_DIR}/bus" DOCKER_HOST="unix://${PODMAN_SOCK}" "$@"
    else
        # Fail fast: a daily CLI must never block on a password prompt. The
        # install-time interactive escalation (run_privileged) is in the
        # installer, not here.
        echo -e "${RED}Error: run this CLI as '${GOPOD_USER:-the GOPOD service user}' or root.${NC}" >&2
        return 1
    fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Runtime mode detection — the CLI adapts to whatever shape is actually
# installed instead of assuming the mode used during installation.
# ─────────────────────────────────────────────────────────────────────────────
DETECTED_MODE="unknown"

detect_mode() {
    if run_rootless systemctl --user list-unit-files 2>/dev/null | grep -q '^gopod-pod\.service'; then
        DETECTED_MODE="quadlet-pod"
    elif run_rootless systemctl --user list-unit-files 2>/dev/null | grep -qE '^gopod(-server|-caddy)\.service'; then
        DETECTED_MODE="quadlet-standalone"
    elif run_rootless podman pod ls --format '{{.Name}}' 2>/dev/null | grep -qE '^gopod(-system)?$'; then
        DETECTED_MODE="pod"
    elif [ -f "$COMPOSE_FILE" ]; then
        DETECTED_MODE="compose"
    elif run_rootless podman ps -a --format '{{.Names}}' 2>/dev/null | grep -q '^gopod$'; then
        DETECTED_MODE="container"
    else
        DETECTED_MODE="unknown"
    fi
}

mode_label() {
    case "$1" in
        quadlet-pod)       echo "Quadlet pod (systemd user units)" ;;
        quadlet-standalone) echo "Quadlet standalone (systemd user units)" ;;
        pod)               echo "Bare podman pod (no systemd)" ;;
        compose)           echo "Podman compose stack" ;;
        container)         echo "Bare podman containers (no systemd)" ;;
        unknown)           echo "Not detected — commands assume manual podman" ;;
    esac
}

# Quadlet unit list actually present (pod wrapper may be absent). Excludes network units.
quadlet_units() {
    run_rootless systemctl --user list-unit-files 2>/dev/null | grep -oE '^gopod[a-z-]*\.service' | grep -v -- '-network\.service$' | sort -u
}

# Start / stop / restart one or many units depending on the detected shape.
start_stack() {
    case "$DETECTED_MODE" in
        quadlet-pod)        run_rootless systemctl --user start gopod-pod.service ;;
        quadlet-standalone) run_rootless systemctl --user start gopod-caddy.service gopod-server.service ;;
        pod)                run_rootless podman pod start gopod ;;
        compose)            run_compose up -d ;;
        container)          run_rootless podman start gopod gopod-caddy ;;
        *)                  echo -e "${RED}No GOPOD stack detected to start.${NC}" >&2; return 1 ;;
    esac
}

stop_stack() {
    case "$DETECTED_MODE" in
        quadlet-pod)        run_rootless systemctl --user stop gopod-pod.service ;;
        quadlet-standalone) run_rootless systemctl --user stop gopod-server.service gopod-caddy.service ;;
        pod)                run_rootless podman pod stop gopod ;;
        compose)            run_compose stop ;;
        container)          run_rootless podman stop gopod gopod-caddy ;;
        *)                  echo -e "${RED}No GOPOD stack detected to stop.${NC}" >&2; return 1 ;;
    esac
}

restart_stack() {
    case "$DETECTED_MODE" in
        quadlet-pod)        run_rootless systemctl --user restart gopod-pod.service ;;
        quadlet-standalone) run_rootless systemctl --user restart gopod-caddy.service gopod-server.service ;;
        pod)                run_rootless podman pod restart gopod ;;
        compose)            run_compose restart ;;
        container)          run_rootless podman restart gopod gopod-caddy ;;
        *)                  echo -e "${RED}No GOPOD stack detected to restart.${NC}" >&2; return 1 ;;
    esac
}

run_compose() {
    local action="$1"
    shift || true

    if run_rootless podman compose version &>/dev/null; then
        run_rootless podman compose -f "$COMPOSE_FILE" "$action" "$@"
    elif command -v podman-compose &>/dev/null; then
        run_rootless podman-compose -f "$COMPOSE_FILE" "$action" "$@"
    elif command -v docker-compose &>/dev/null; then
        run_rootless docker-compose -f "$COMPOSE_FILE" "$action" "$@"
    else
        case "$action" in
            logs)
                run_rootless podman logs "$@" gopod
                ;;
            restart)
                run_rootless podman restart gopod gopod-caddy
                ;;
            stop)
                run_rootless podman stop gopod gopod-caddy
                ;;
            start|up)
                run_rootless podman start gopod gopod-caddy
                ;;
            ps)
                run_rootless podman ps -a --filter name=gopod
                ;;
            pull)
                run_rootless podman pull ghcr.io/ijon6k/gopod:latest 2>/dev/null || true
                ;;
            *)
                echo -e "${RED}Error: Compose command not found and action '${action}' not supported in fallback.${NC}" >&2
                return 1
                ;;
        esac
    fi
}

# Caddy Admin API helper (unix domain socket — universal for every install
# shape: pod, standalone quadlet, compose, bare metal).
admin_api() {
    local method="$1"
    local path="$2"
    if [ ! -S "$ADMIN_SOCK" ]; then
        echo -e "${RED}Admin socket not found at: ${ADMIN_SOCK}${NC}" >&2
        echo "Is the Caddy ingress running? Check: gopod ps" >&2
        return 2
    fi
    curl -fsS --max-time 5 --unix-socket "$ADMIN_SOCK" -X "$method" "http://unix${path}"
}

show_help() {
    cat << EOF
${BLUE}${BOLD}GOPOD CLI${NC} — Universal Management Helper for Rootless GOPOD PaaS
${DIM}Every command adapts automatically to how GOPOD is deployed${NC}

${BOLD}Usage:${NC}
  gopod <command> [arguments]

${BOLD}Core Commands (work in every deployment mode):${NC}
  ${CYAN}start${NC}                 Start the GOPOD stack
  ${CYAN}stop${NC}                  Stop the GOPOD stack
  ${CYAN}restart${NC} [service]     Restart whole stack or one service (server|caddy)
  ${CYAN}status${NC}                Show deployment mode, health, socket and containers
  ${CYAN}ps${NC}                    List GOPOD containers
  ${CYAN}logs${NC} [-f] [server|caddy]
                             View or stream logs (default: server)
  ${CYAN}shell${NC} [caddy]         Open interactive shell in a container
  ${CYAN}exec${NC} <ctr> <cmd...>   Run a command inside a container
  ${CYAN}update${NC}                Pull latest images and restart the stack
  ${CYAN}info${NC}                  Show paths, ports, sockets and mode
  ${CYAN}podman${NC} <args...>      Run any Podman command in rootless context

${BOLD}Systemd / Quadlet Helpers:${NC}
  ${CYAN}units list${NC}            List installed GOPOD systemd user units
  ${CYAN}units start${NC}           Start all GOPOD units
  ${CYAN}units stop${NC}            Stop all GOPOD units
  ${CYAN}units restart${NC}         Restart all GOPOD units
  ${CYAN}units status${NC}          Show systemd status of GOPOD units
  ${CYAN}daemon-reload${NC}         Reload systemd user manager (after unit edits)

${BOLD}Caddy Admin API (via unix socket, no TCP):${NC}
  ${CYAN}caddy reload${NC}          Re-read the Caddyfile from disk (zero-downtime)
  ${CYAN}caddy status${NC}          Check the admin socket and dump active config
  ${CYAN}caddy errors${NC}          Return the last reload error message

${BOLD}Context:${NC}
  Service User : ${BOLD}${GOPOD_USER}${NC} (UID: ${GOPOD_UID})
  Deployment   : detected at runtime via ${DIM}detect_mode${NC}
  Socket       : ${DIM}${PODMAN_SOCK}${NC}
  Admin Socket : ${DIM}${ADMIN_SOCK}${NC}
  Dashboard    : ${BOLD}http://localhost:${ACTUAL_PORT}${NC}
EOF
}

# ─────────────────────────────────────────────────────────────────────────────
# Main dispatch
# ─────────────────────────────────────────────────────────────────────────────
CMD="${1:-help}"
shift || true

case "$CMD" in
    start)
        detect_mode
        ensure_socket
        echo -e "${CYAN}==>${NC} Starting GOPOD (${DIM}$(mode_label "$DETECTED_MODE")${NC})..."
        start_stack
        ;;

    stop)
        detect_mode
        stop_stack
        ;;

    restart)
        target="${1:-}"
        detect_mode
        ensure_socket
        if [ -n "$target" ]; then
            case "$target" in
                caddy|gopod-caddy) run_rootless podman restart gopod-caddy ;;
                server|app|gopod)  run_rootless podman restart gopod ;;
                *)                 run_rootless podman restart "$target" ;;
            esac
        else
            echo -e "${CYAN}==>${NC} Restarting GOPOD (${DIM}$(mode_label "$DETECTED_MODE")${NC})..."
            restart_stack
        fi
        ;;

    status)
        detect_mode
        echo -e "${BLUE}${BOLD}=== GOPOD Service Status ===${NC}"
        echo -e "  Running Mode : ${CYAN}$(mode_label "$DETECTED_MODE")${NC}"

        if curl -fsSL -o /dev/null "http://127.0.0.1:${ACTUAL_PORT}" 2>/dev/null; then
            echo -e "  Health Check : ${GREEN}${BOLD}Healthy${NC} (HTTP 200 on :${ACTUAL_PORT})"
        else
            echo -e "  Health Check : ${RED}${BOLD}Unreachable / Starting${NC} (default :${ACTUAL_PORT})"
        fi

        ensure_socket
        if [ -S "$PODMAN_SOCK" ] || run_rootless test -S "$PODMAN_SOCK" 2>/dev/null; then
            echo -e "  Podman Socket: ${GREEN}Active${NC} (${PODMAN_SOCK})"
        else
            echo -e "  Podman Socket: ${YELLOW}Not found or inactive${NC} (${PODMAN_SOCK})"
        fi

        if [ -S "$ADMIN_SOCK" ]; then
            echo -e "  Caddy Admin  : ${GREEN}Socket present${NC} (${ADMIN_SOCK})"
        else
            echo -e "  Caddy Admin  : ${YELLOW}Socket absent${NC} (${ADMIN_SOCK})"
        fi

        echo ""
        run_rootless podman ps -a --filter name=gopod --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        ;;

    ps)
        ensure_socket
        run_rootless podman ps -a --filter name=gopod "$@"
        ;;

    logs)
        ensure_socket
        target="gopod"
        # Normalize staffing flags: `-n N` isn't a podman logs option (docker
        # spelling); rewrite it to podman's `--tail N` transparently.
        logargs=()
        while [ $# -gt 0 ]; do
            case "$1" in
                -n|--tail)
                    shift || true
                    logargs+=(--tail "${1:-50}")
                    ;;
                *) logargs+=("$1") ;;
            esac
            shift || true
        done
        set -- "${logargs[@]}"
        if [ "${1:-}" = "caddy" ] || [ "${1:-}" = "gopod-caddy" ]; then
            target="gopod-caddy"
            shift || true
        elif [ "${1:-}" = "server" ] || [ "${1:-}" = "gopod" ] || [ "${1:-}" = "app" ]; then
            target="gopod"
            shift || true
        fi
        if [ "$DETECTED_MODE" = "unknown" ]; then detect_mode; fi
        if [ "$DETECTED_MODE" = "compose" ]; then
            run_compose logs "$@"
        else
            run_rootless podman logs "$@" "$target"
        fi
        ;;

    shell)
        ensure_socket
        target="${1:-server}"
        case "$target" in
            caddy|gopod-caddy) target="gopod-caddy" ;;
            server|app|gopod)  target="gopod" ;;
        esac
        if [ -t 0 ] && [ -t 1 ]; then
            run_rootless podman exec -it "$target" sh -c "exec bash 2>/dev/null || exec sh"
        else
            run_rootless podman exec -i "$target" sh -c "exec bash 2>/dev/null || exec sh"
        fi
        ;;

    exec)
        ensure_socket
        if [ -t 0 ] && [ -t 1 ]; then
            run_rootless podman exec -it "$@"
        else
            run_rootless podman exec -i "$@"
        fi
        ;;

    update)
        detect_mode
        ensure_socket
        echo -e "${CYAN}==>${NC} Pulling latest GOPOD images..."
        case "$DETECTED_MODE" in
            quadlet-pod)
                run_rootless podman auto-update
                echo -e "${CYAN}==>${NC} Restarting GOPOD pod..."
                run_rootless systemctl --user restart gopod-pod.service 2>/dev/null || true
                ;;
            quadlet-standalone)
                run_rootless podman auto-update
                echo -e "${CYAN}==>${NC} Restarting units..."
                quadlet_units | while read -r u; do
                    run_rootless systemctl --user restart "$u" 2>/dev/null || true
                done
                ;;
            pod)
                run_rootless podman pull ghcr.io/ijon6k/gopod:latest 2>/dev/null || true
                restart_stack
                ;;
            compose)
                run_compose pull
                echo -e "${CYAN}==>${NC} Recreating GOPOD services..."
                run_compose up -d
                ;;
            container)
                run_rootless podman pull ghcr.io/ijon6k/gopod:latest 2>/dev/null || true
                restart_stack
                ;;
            *)
                echo -e "${RED}No GOPOD stack detected to update.${NC}" >&2
                exit 1
                ;;
        esac
        ;;

    podman)
        ensure_socket
        run_rootless podman "$@"
        ;;

    units)
        detect_mode
        SUBCMD="${1:-list}"
        shift || true
        case "$SUBCMD" in
            list)
                run_rootless systemctl --user list-unit-files 'gopod*' --no-pager
                ;;
            start)
                if [ "$DETECTED_MODE" = "quadlet-pod" ]; then
                    echo -e "  ${CYAN}==>${NC} start gopod-pod.service"
                    run_rootless systemctl --user start gopod-pod.service
                else
                    quadlet_units | while read -r u; do
                        echo -e "  ${CYAN}==>${NC} start ${u}"
                        run_rootless systemctl --user start "$u"
                    done
                fi
                ;;
            stop)
                if [ "$DETECTED_MODE" = "quadlet-pod" ]; then
                    echo -e "  ${CYAN}==>${NC} stop gopod-pod.service"
                    run_rootless systemctl --user stop gopod-pod.service
                else
                    quadlet_units | while read -r u; do
                        echo -e "  ${CYAN}==>${NC} stop ${u}"
                        run_rootless systemctl --user stop "$u" 2>/dev/null || true
                    done
                fi
                ;;
            restart)
                if [ "$DETECTED_MODE" = "quadlet-pod" ]; then
                    echo -e "  ${CYAN}==>${NC} restart gopod-pod.service"
                    run_rootless systemctl --user restart gopod-pod.service
                else
                    quadlet_units | while read -r u; do
                        echo -e "  ${CYAN}==>${NC} restart ${u}"
                        run_rootless systemctl --user restart "$u" 2>/dev/null || true
                    done
                fi
                ;;
            status)
                run_rootless systemctl --user status gopod-pod.service gopod-server.service gopod-caddy.service --no-pager 2>/dev/null || \
                run_rootless systemctl --user status --no-pager "$(quadlet_units | tr '\n' ' ')" 2>/dev/null
                ;;
            *)
                echo -e "${RED}Unknown units command '${SUBCMD}'. Available: list|start|stop|restart|status${NC}" >&2
                exit 1
                ;;
        esac
        ;;

    daemon-reload)
        echo -e "${CYAN}==>${NC} Reloading systemd user units..."
        run_rootless systemctl --user daemon-reload && echo -e "  ${GREEN}✓${NC} User manager reloaded"
        ;;

    caddy)
        SUBCMD="${1:-status}"
        shift || true
        case "$SUBCMD" in
            reload)
                # Zero-downtime: Caddy re-reads the Caddyfile from disk.
                if admin_api POST "/reload"; then
                    echo -e "  ${GREEN}✓${NC} Caddy reloaded (${ADMIN_SOCK})"
                else
                    echo -e "  ${RED}✗${NC} Reload failed"
                    exit 1
                fi
                ;;
            status)
                if admin_api GET "/config/" > /dev/null 2>&1; then
                    echo -e "  ${GREEN}✓${NC} Caddy admin API reachable via ${BOLD}${ADMIN_SOCK}${NC}"
                    echo -e "  ${DIM}TCP admin port: not exposed (unix socket transport)${NC}"
                else
                    echo -e "  ${RED}✗${NC} Caddy admin API unreachable via ${ADMIN_SOCK}"
                    exit 1
                fi
                ;;
            errors)
                # /config/ with a guarded exit: any 4xx renders Caddy's message.
                if response="$(admin_api GET "/config/" 2>&1)"; then
                    echo -e "  ${GREEN}✓${NC} No pending config errors"
                else
                    echo -e "  ${YELLOW}⚠${NC} ${response}"
                    exit 1
                fi
                ;;
            *)
                echo -e "${RED}Unknown caddy command '${SUBCMD}'. Available: reload|status|errors${NC}" >&2
                exit 1
                ;;
        esac
        ;;

    info)
        detect_mode
        cat << EOF
${BLUE}${BOLD}=== GOPOD System & Environment Info ===${NC}
  Service User : ${BOLD}${GOPOD_USER}${NC} (UID: ${GOPOD_UID})
  Deploy Mode  : ${BOLD}$(mode_label "$DETECTED_MODE")${NC}
  Home Dir     : ${DIM}${GOPOD_HOME}${NC}
  Config Dir   : ${DIM}${GOPOD_CONFIG}${NC}
  Data Dir     : ${DIM}${GOPOD_DATA}${NC}
  Compose File : ${DIM}${COMPOSE_FILE}${NC}
  Quadlet Dir  : ${DIM}${QUADLET_DIR}${NC}
  Podman Socket: ${DIM}${PODMAN_SOCK}${NC}
  Admin Socket : ${DIM}${ADMIN_SOCK}${NC}
  Dashboard URL: ${BOLD}http://localhost:${ACTUAL_PORT}${NC}
  Ingress HTTP : ${BOLD}http://localhost:${ACTUAL_CADDY_PORT}${NC}
EOF
        ;;

    help|-h|--help)
        show_help
        ;;

    *)
        echo -e "${RED}Unknown command: '${CMD}'${NC}\n" >&2
        show_help >&2
        exit 1
        ;;
esac
WRAPPER_BODY

    # Install wrapper to /usr/local/bin/gopod
    chmod 0755 "$tmp_wrapper"
    if [ "$(id -u)" -eq 0 ]; then
        mkdir -p /usr/local/bin 2>/dev/null || true
        cp "$tmp_wrapper" /usr/local/bin/gopod 2>/dev/null || true
        chmod 0755 /usr/local/bin/gopod 2>/dev/null || true
        ln -sf /usr/local/bin/gopod /usr/bin/gopod 2>/dev/null || true
    elif command -v sudo &>/dev/null && sudo -n true 2>/dev/null; then
        sudo mkdir -p /usr/local/bin 2>/dev/null || true
        sudo cp "$tmp_wrapper" /usr/local/bin/gopod 2>/dev/null || true
        sudo chmod 0755 /usr/local/bin/gopod 2>/dev/null || true
        sudo ln -sf /usr/local/bin/gopod /usr/bin/gopod 2>/dev/null || true
    elif [ -w /usr/local/bin ]; then
        cp "$tmp_wrapper" /usr/local/bin/gopod 2>/dev/null || true
        chmod 0755 /usr/local/bin/gopod 2>/dev/null || true
    fi

    # Also install to ~/.local/bin/gopod for the user
    mkdir -p "${wrapper_home}/.local/bin" 2>/dev/null || true
    cp "$tmp_wrapper" "${wrapper_home}/.local/bin/gopod" 2>/dev/null || true
    chmod 0755 "${wrapper_home}/.local/bin/gopod" 2>/dev/null || true
    if [ "$(id -u)" -eq 0 ] && [ "$wrapper_user" != "root" ]; then
        chown -R "${wrapper_user}:${wrapper_user}" "${wrapper_home}/.local" 2>/dev/null || true
    fi

    rm -f "$tmp_wrapper" 2>/dev/null || true
    return 0
}

# ─────────────────────────────────────────────────────────────────────────────
# Default Configuration Values & Parameter Parsing
# ─────────────────────────────────────────────────────────────────────────────

ORIGINAL_ARGS=("$@")

CURRENT_USER=$(id -un)
USER_UID=$(id -u)
USER_HOME=$(eval echo "~${CURRENT_USER}")

RUN_MODE="quadlet-pod"        # Default: systemd quadlet inside a pod. Alternatives: quadlet-standalone / container.
CADDY_ADMIN_MODE="unix"       # Recommended default: filesystem socket, no TCP. Override with CADDY_ADMIN_URL env.
RELEASE_CHANNEL="latest"      # Default: latest. Optional: dev / beta.
CUSTOM_TAG=""                 # Specific tag or sha (overrides channel).
GOPOD_PORT="8085"             # Dashboard port
CADDY_HTTP_PORT="80"          # Ingress HTTP
CADDY_HTTPS_PORT="443"        # Ingress HTTPS
EXPLICIT_PORT_OVERRIDE=false  # Set true when the operator passes port flags
CONFIG_DIR="${USER_HOME}/.config/gopod"
DATA_DIR="${USER_HOME}/.local/share/gopod"
INTERACTIVE=false
NON_INTERACTIVE=false
DRY_RUN=false
VERBOSE=false
ALLOW_ROOT=false
AS_USER=false
SERVICE_USER="gopod"

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
  --mode=container      Install as standard Podman Compose container service
  --mode=quadlet-pod    Install as native systemd user units, gopod & caddy inside one pod (Default, Recommended).
                        Uses a unix-scope Caddy Admin socket (no TCP admin port).
  --mode=quadlet        Alias of --mode=quadlet-pod (legacy shorthand)
  --mode=quadlet-standalone
                        Same as quadlet-pod but without the pod wrapper: two
                        independent systemd user units. Still reachable via the
                        shared unix-scope Admin socket volume.
  --container           Shortcut for --mode=container
  --quadlet             Shortcut for --mode=quadlet-pod

Security & User Context:
  --rootless            Enforce rootless execution (Default on host OS)
  --user=<USERNAME>     Specify dedicated service user for rootless mode (Default: gopod)
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
            EXPLICIT_PORT_OVERRIDE=true
            ;;
        --port)
            GOPOD_PORT="$2"; shift
            EXPLICIT_PORT_OVERRIDE=true
            ;;
        --http-port=*)
            CADDY_HTTP_PORT="${1#*=}"
            EXPLICIT_PORT_OVERRIDE=true
            ;;
        --http-port)
            CADDY_HTTP_PORT="$2"; shift
            EXPLICIT_PORT_OVERRIDE=true
            ;;
        --https-port=*)
            CADDY_HTTPS_PORT="${1#*=}"
            EXPLICIT_PORT_OVERRIDE=true
            ;;
        --https-port)
            CADDY_HTTPS_PORT="$2"; shift
            EXPLICIT_PORT_OVERRIDE=true
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
        --user=*)
            SERVICE_USER="${1#*=}"
            ;;
        --user)
            SERVICE_USER="$2"; shift
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
    quadlet|quadlet-pod) RUN_MODE="quadlet-pod" ;;
    quadlet-standalone|standalone) RUN_MODE="quadlet-standalone" ;;
    *)
        echo -e "${RED}Invalid --mode specified: '$RUN_MODE'. Must be 'quadlet-pod', 'quadlet-standalone', or 'container'.${NC}"
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
    if [ "$ALLOW_ROOT" = false ]; then
        if [ "$INTERACTIVE" = true ] && [ "$NON_INTERACTIVE" = false ]; then
            TTY_DEV="/dev/tty"
            if [ -e "$TTY_DEV" ] || [ -t 0 ]; then
                exec 3< "$TTY_DEV" 2>/dev/null || exec 3<&0
                echo -e "${CYAN}${BOLD}"
                echo "┌─────────────────────────────────────────────────────────────┐"
                echo "│              Privilege & Security Context                   │"
                echo "└─────────────────────────────────────────────────────────────┘"
                echo -e "${NC}"
                echo -e "Installer running as root (UID 0)."
                echo -e "Choose privilege execution mode:\n"
                echo -e "  ${BOLD}1)${NC} ${GREEN}Rootless (Recommended)${NC} — Dedicated unprivileged user (PaaS standard)"
                echo -e "  ${BOLD}2)${NC} ${YELLOW}Rootful (Advanced)${NC}     — Direct UID 0 (WireGuard, eBPF, raw block storage)\n"
                read -r -u 3 -p "Select mode [1/2] (default: 1): " CHOICE_PRIV
                case "$CHOICE_PRIV" in
                    2|rootful|root)
                        ALLOW_ROOT=true
                        echo -e "\n  ${YELLOW}${CHAR_WARN} Selected Rootful mode (UID 0).${NC}\n"
                        ;;
                    *)
                        ALLOW_ROOT=false
                        read -r -u 3 -p "Enter service username [${SERVICE_USER}]: " INPUT_USER
                        if [ -n "$INPUT_USER" ]; then
                            if [[ "$INPUT_USER" =~ ^[a-z_][a-z0-9_-]*$ ]]; then
                                SERVICE_USER="$INPUT_USER"
                            else
                                echo -e "${YELLOW}Invalid username format. Using default '${SERVICE_USER}'.${NC}"
                            fi
                        fi
                        ;;
                esac
                exec 3<&-
            fi
        fi

        if [ "$ALLOW_ROOT" = false ]; then
            echo -e "\n${CYAN}${BOLD}==> Provisioning dedicated '${SERVICE_USER}' service user for rootless execution...${NC}"

            # 1. Create service user if missing
            if ! id -u "$SERVICE_USER" &>/dev/null; then
                useradd -m -s /bin/bash "$SERVICE_USER" 2>/dev/null || adduser -D -s /bin/bash "$SERVICE_USER" 2>/dev/null
                echo -e "  ${GREEN}${CHAR_CHECK}${NC} Created dedicated service user: ${BOLD}${SERVICE_USER}${NC}"
            else
                echo -e "  ${GREEN}${CHAR_CHECK}${NC} Using existing service user: ${BOLD}${SERVICE_USER}${NC}"
            fi

            SERVICE_HOME=$(eval echo "~${SERVICE_USER}")
            GOPOD_SERVICE_UID=$(id -u "$SERVICE_USER")

            # Ensure service home exists with proper base ownership
            mkdir -p "$SERVICE_HOME"
            chown -R "${SERVICE_USER}:${SERVICE_USER}" "$SERVICE_HOME" 2>/dev/null || true
            chmod 750 "$SERVICE_HOME" 2>/dev/null || true

            # 2. Configure subordinate UID/GID mappings
            usermod --add-subuids 100000-165535 "$SERVICE_USER" 2>/dev/null || true
            usermod --add-subgids 100000-165535 "$SERVICE_USER" 2>/dev/null || true
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
                loginctl enable-linger "$SERVICE_USER" 2>/dev/null || true
                echo -e "  ${GREEN}${CHAR_CHECK}${NC} Systemd process lingering enabled for '${SERVICE_USER}'"
            fi

            # 5. Install packages as root before delegating
            echo -e "  ${CYAN}==>${NC} Validating host system packages..."
            install_packages_as_root

            # 6. Ensure runtime directory
            GOPOD_RUNTIME_DIR="/run/user/${GOPOD_SERVICE_UID}"
            mkdir -p "$GOPOD_RUNTIME_DIR" 2>/dev/null || true
            chown -R "${SERVICE_USER}:${SERVICE_USER}" "$GOPOD_RUNTIME_DIR" 2>/dev/null || true
            chmod 700 "$GOPOD_RUNTIME_DIR" 2>/dev/null || true

            # Activate systemd user manager if available
            if command -v systemctl &>/dev/null && [ -d /run/systemd/system ]; then
                systemctl start "user@${GOPOD_SERVICE_UID}.service" 2>/dev/null || true
            fi

            # Provision CLI wrapper early as root
            create_cli_wrapper "$SERVICE_USER" "$GOPOD_SERVICE_UID" "$SERVICE_HOME" "${SERVICE_HOME}/.config/gopod" "${SERVICE_HOME}/.local/share/gopod" "$GOPOD_PORT" "$CADDY_HTTP_PORT"

            # 7. Re-execute script under service user
            SCRIPT_TEMP="/tmp/gopod-installer-$$.sh"
            if [ -f "$0" ] && [ -r "$0" ]; then
                cp "$0" "$SCRIPT_TEMP"
            else
                curl -fsSL "https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh" -o "$SCRIPT_TEMP"
            fi
            chmod 755 "$SCRIPT_TEMP"

            # Ensure entire service home is owned by service user before dropping privileges
            chown -R "${SERVICE_USER}:${SERVICE_USER}" "$SERVICE_HOME" 2>/dev/null || true

            echo -e "  ${GREEN}${CHAR_CHECK}${NC} Switching execution to user '${BOLD}${SERVICE_USER}${NC}'...\n"
            su - "$SERVICE_USER" -c "XDG_RUNTIME_DIR=${GOPOD_RUNTIME_DIR} DBUS_SESSION_BUS_ADDRESS=unix:path=${GOPOD_RUNTIME_DIR}/bus bash ${SCRIPT_TEMP} ${ORIGINAL_ARGS[*]} --as-user"
            SUB_EXIT=$?
            rm -f "$SCRIPT_TEMP" 2>/dev/null || true
            if [ $SUB_EXIT -eq 0 ]; then
                create_cli_wrapper "$SERVICE_USER" "$GOPOD_SERVICE_UID" "$SERVICE_HOME" "${SERVICE_HOME}/.config/gopod" "${SERVICE_HOME}/.local/share/gopod" "$GOPOD_PORT" "$CADDY_HTTP_PORT"
            fi
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
        echo -e "  ${BOLD}1)${NC} ${GREEN}Quadlet Pod (systemd user pod)${NC} [Default, Recommended] — gopod + caddy share pod; unix-scope admin socket"
        echo -e "  ${BOLD}2)${NC} ${GREEN}Quadlet Standalone (systemd user units)${NC} — no pod wrapper; still uses shared admin socket volume"
        echo -e "  ${BOLD}3)${NC} ${BLUE}Container (Podman Compose)${NC} — two bridge networks, caddy bridges tenants & control plane"
        read -r -u 3 -p "Select mode [1/2/3] (default: 1): " CHOICE_MODE
        case "$CHOICE_MODE" in
            2|standalone|s)
                RUN_MODE="quadlet-standalone"
                ;;
            3|container|c|compose)
                RUN_MODE="container"
                ;;
            *)
                RUN_MODE="quadlet-pod"
                ;;
        esac

        # 3. Ports
        echo -e "\n${BOLD}[3/4] Port Configuration:${NC}"
        read -r -u 3 -p "Web Dashboard Port [${GOPOD_PORT}]: " INPUT_PORT
        if [ -n "$INPUT_PORT" ]; then
            GOPOD_PORT="$INPUT_PORT"
            EXPLICIT_PORT_OVERRIDE=true
        fi

        read -r -u 3 -p "Caddy HTTP Ingress Port [${CADDY_HTTP_PORT}]: " INPUT_HTTP
        if [ -n "$INPUT_HTTP" ]; then
            CADDY_HTTP_PORT="$INPUT_HTTP"
            EXPLICIT_PORT_OVERRIDE=true
        fi

        read -r -u 3 -p "Caddy HTTPS Ingress Port [${CADDY_HTTPS_PORT}]: " INPUT_HTTPS
        if [ -n "$INPUT_HTTPS" ]; then
            CADDY_HTTPS_PORT="$INPUT_HTTPS"
            EXPLICIT_PORT_OVERRIDE=true
        fi

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
echo -e " Deploy Mode  : ${BOLD}${RUN_MODE}${NC} $([ "$RUN_MODE" = "quadlet-pod" ] && echo "(Default - Recommended)" || ([ "$RUN_MODE" = "quadlet-standalone" ] && echo "(Independent systemd units)" || echo "(Container/Compose stack)"))"
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
        # Re-verify: rootless podman cannot run without subordinate UID/GID
        # ranges, so a silent failure here must stop with an actionable
        # message instead of surfacing 200 lines later inside podman.
        if ! grep -q "^${CURRENT_USER}:" /etc/subuid 2>/dev/null || ! grep -q "^${CURRENT_USER}:" /etc/subgid 2>/dev/null; then
            echo -e "${RED}Error: user '${CURRENT_USER}' has no subordinate UID/GID ranges in /etc/subuid, /etc/subgid.${NC}" >&2
            echo -e "${YELLOW}Rootless Podman requires them. Rerun with sudo once, or as root:${NC}" >&2
            echo -e "  usermod --add-subuids 100000-165535 --add-subgids 100000-165535 ${CURRENT_USER}" >&2
            return 1
        fi
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

# Rootless port sanity: without the sysctl above (needs root/sudo) a plain
# user cannot bind ports < 1024. PublishPort on a rootless pod would fail at
# start with a confusing binding error, so fall back to high ports that
# always work — but only when the ports are still installer defaults, never
# when the operator explicitly picked them.
unpriv_port_start=$(cat /proc/sys/net/ipv4/ip_unprivileged_port_start 2>/dev/null || echo "1024")
if [ "$(id -u)" -ne 0 ] \
    && ! command -v sudo &>/dev/null && ! command -v doas &>/dev/null \
    && [ "$EXPLICIT_PORT_OVERRIDE" != "true" ] \
    && { [ "${CADDY_HTTP_PORT:-80}" -lt "$unpriv_port_start" ] || [ "${CADDY_HTTPS_PORT:-443}" -lt "$unpriv_port_start" ]; }; then
    echo -e "${YELLOW}Ports 80/443 will not be bindable rootlessly (no sysctl change possible without root).${NC}"
    echo -e "${YELLOW}Falling back to high ports: HTTP 8080, HTTPS 8443.${NC}"
    echo -e "${DIM}To keep 80/443, rerun with root/sudo once, or apply: sysctl net.ipv4.ip_unprivileged_port_start=80${NC}"
    CADDY_HTTP_PORT="8080"
    CADDY_HTTPS_PORT="8443"
fi

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

    # Dashboard upstream depends on the network topology: inside a quadlet
    # pod, gopod & caddy share a netns, so loopback applies. Standalone
    # quadlet and compose live in separate netns joined by gopod-internal,
    # so DNS names are required.
    local caddy_upstream="gopod:8085"
    if [ "$RUN_MODE" = "quadlet-pod" ]; then
        caddy_upstream="127.0.0.1:8085"
    fi

    if [ ! -f "${GOPOD_CONFIG}/caddy/Caddyfile" ]; then
        cat <<EOF > "${GOPOD_CONFIG}/caddy/Caddyfile"
{
    admin unix//run/gopod-caddy/admin.sock
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
    mkdir -p "${GOPOD_DATA}/caddy-admin"

    chmod 750 "${GOPOD_DATA}" "${GOPOD_CONFIG}" 2>/dev/null || true
    chmod 700 "${GOPOD_DATA}/keys" 2>/dev/null || true
    # Caddy process must be able to create the socket in this dir (rootless
    # uid runs the Caddy container: only the service user owns it).
    chmod 755 "${GOPOD_DATA}/caddy-admin" 2>/dev/null || true
    chmod 644 "${GOPOD_DATA}/logs/access.log" 2>/dev/null || true

    if [ "$SELINUX_STATUS" = "enforcing" ] || [ "$SELINUX_STATUS" = "permissive" ]; then
        if command -v chcon &> /dev/null; then
            chcon -Rt container_file_t "${GOPOD_DATA}" "${GOPOD_CONFIG}" 2>/dev/null || true
        fi
    fi
}
run_task "Creating configuration and persistent data directories" task_init_storage

# ─────────────────────────────────────────────────────────────────────────────
# [6/7] Pulling Container Images (Realtime Progress)
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 6 "Pulling container images (${DISPLAY_TAG})"

task_pull_gopod() {
    podman pull "$GOPOD_IMAGE"
}
run_live_task "Downloading GOPOD control plane (${GOPOD_IMAGE})" task_pull_gopod

task_pull_caddy() {
    podman pull "$CADDY_IMAGE"
}
run_live_task "Downloading Caddy reverse proxy ingress (${CADDY_IMAGE})" task_pull_caddy

# ─────────────────────────────────────────────────────────────────────────────
# [7/7] Generating Manifests & Starting Service
# ─────────────────────────────────────────────────────────────────────────────
render_progress_bar 7 "Orchestrating GOPOD services (${RUN_MODE} mode)"

task_generate_manifests() {
    # Volume dir shared between the control plane and Caddy, holding the
    # unix-scope Caddy Admin API socket (no TCP admin port in any mode).
    local CADDY_ADMIN_SOCK_DIR="${GOPOD_DATA}/caddy-admin"

    # Compose manifest is only generated for the container deployment shape;
    # quadlet modes manage state through systemd units instead.
    if [ "$RUN_MODE" = "container" ]; then
    # Generate Podman Compose manifest (dual networks: control plane isolated
    # from tenant gopod-net; Caddy bridges both).
    cat <<EOF > "${GOPOD_CONFIG}/compose.yml"
services:
  gopod:
    image: ${GOPOD_IMAGE}
    container_name: gopod
    restart: always
    environment:
      - PORT=8085
      - PODMAN_SOCKET=/run/podman/podman.sock
      - CONTAINER_HOST=unix:///run/podman/podman.sock
      - DOCKER_HOST=unix:///run/podman/podman.sock
      - GOPOD_DB_PATH=/app/data/gopod.db
      - CADDY_ADMIN_URL=unix:///run/gopod-caddy/admin.sock
    volumes:
      - "${DETECTED_SOCKET}:/run/podman/podman.sock:ro,z"
      - "${GOPOD_DATA}/data:/app/data:Z"
      - "${GOPOD_DATA}/logs:/var/log/caddy:z"
      - "${GOPOD_DATA}/caddy-admin:/run/gopod-caddy:z"
    networks:
      - gopod-internal
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
      - "${GOPOD_DATA}/caddy-admin:/run/gopod-caddy:z"
    networks:
      - gopod-internal
      - gopod-net

networks:
  gopod-internal:
    name: gopod-internal
    driver: bridge
  gopod-net:
    name: gopod-net
    driver: bridge
EOF
        rm -f "${QUADLET_DIR}/gopod.pod" "${QUADLET_DIR}/gopod-caddy.container" "${QUADLET_DIR}/gopod-server.container" 2>/dev/null || true
        return 0
    else
        rm -f "${GOPOD_CONFIG}/compose.yml" 2>/dev/null || true
    fi

    # Generate Systemd Quadlet manifests
    cat <<EOF > "${QUADLET_DIR}/gopod-net.network"
[Network]
NetworkName=gopod-net
Subnet=10.89.0.0/16
Gateway=10.89.0.1
Internal=false
EOF

    if [ "$RUN_MODE" = "quadlet-pod" ]; then
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
    else
        rm -f "${QUADLET_DIR}/gopod.pod" 2>/dev/null || true
    fi

    local POD_JOIN=""
    local POD_WANT=""
    local CADDY_STANDALONE_NET=""
    local SERVER_STANDALONE_NET=""
    if [ "$RUN_MODE" = "quadlet-pod" ]; then
        POD_JOIN="Pod=gopod.pod"
        POD_WANT="PartOf=gopod-pod.service"
    elif [ "$RUN_MODE" = "quadlet-standalone" ]; then
        POD_WANT="After=network-online.target"
        CADDY_STANDALONE_NET="Network=gopod-net.network
PublishPort=${CADDY_HTTP_PORT}:80
PublishPort=${CADDY_HTTPS_PORT}:443"
        SERVER_STANDALONE_NET="Network=gopod-net.network
PublishPort=${GOPOD_PORT}:8085"
    fi

    cat <<EOF > "${QUADLET_DIR}/gopod-caddy.container"
[Unit]
Description=GOPOD Caddy Ingress Reverse Proxy
${POD_WANT}

[Container]
${POD_JOIN}
${CADDY_STANDALONE_NET}
ContainerName=gopod-caddy
Image=${CADDY_IMAGE}
AutoUpdate=registry
Volume=${GOPOD_CONFIG}/caddy/Caddyfile:/etc/caddy/Caddyfile:Z
Volume=${GOPOD_DATA}/caddy/data:/data:Z
Volume=${GOPOD_DATA}/caddy/config:/config:Z
Volume=${GOPOD_DATA}/logs:/var/log/caddy:z
Volume=${GOPOD_DATA}/caddy-admin:/run/gopod-caddy:z
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
${POD_WANT}
After=gopod-caddy.service

[Container]
${POD_JOIN}
${SERVER_STANDALONE_NET}
ContainerName=gopod
Image=${GOPOD_IMAGE}
AutoUpdate=registry
Environment=PORT=8085
Environment=PODMAN_SOCKET=/run/podman/podman.sock
Environment=GOPOD_DB_PATH=/app/data/gopod.db
Environment=CADDY_ADMIN_URL=unix:///run/gopod-caddy/admin.sock
EOF

    if [ "$RUN_MODE" = "quadlet-pod" ]; then
        cat <<EOF >> "${QUADLET_DIR}/gopod-server.container"
Environment=GOPOD_BIND_HOST=127.0.0.1
EOF
    fi

    cat <<EOF >> "${QUADLET_DIR}/gopod-server.container"
Volume=${DETECTED_SOCKET}:/run/podman/podman.sock:z
Volume=${GOPOD_DATA}/data:/app/data:Z
Volume=${GOPOD_DATA}/logs:/var/log/caddy:z
Volume=${GOPOD_DATA}/caddy-admin:/run/gopod-caddy:z

[Service]
Restart=always
RestartSec=3s

[Install]
WantedBy=default.target
EOF
}

start_quadlet() {
    if ! command -v systemctl &> /dev/null; then return 1; fi
    # Clean up stale standalone or compose containers to prevent name collisions
    podman stop gopod gopod-caddy 2>/dev/null || true
    podman rm gopod gopod-caddy 2>/dev/null || true
    if [ "$RUN_MODE" = "quadlet-standalone" ]; then
        podman pod stop gopod 2>/dev/null || true
        podman pod rm gopod 2>/dev/null || true
        systemctl --user stop gopod-pod.service 2>/dev/null || true
    fi
    systemctl --user daemon-reload 2>/dev/null || return 1
    if [ "$RUN_MODE" = "quadlet-pod" ]; then
        systemctl --user enable --now gopod-pod.service 2>/dev/null || return 1
        sleep 2
        systemctl --user is-active --quiet gopod-pod.service 2>/dev/null
    else
        systemctl --user enable --now gopod-caddy.service gopod-server.service 2>/dev/null || return 1
        sleep 2
        systemctl --user is-active --quiet gopod-server.service 2>/dev/null
    fi
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
        podman network create gopod-internal 2>/dev/null || true
        podman network create gopod-net 2>/dev/null || true
        # Bare-metal fallback: two independent containers sharing the admin
        # socket bind-mount. gopod is not published on :8085 — only Caddy
        # proxies it on gopod-internal.
        podman run -d --name gopod \
            --restart=always \
            --network=gopod-internal \
            -e PORT=8085 \
            -e PODMAN_SOCKET=/run/podman/podman.sock \
            -e GOPOD_DB_PATH=/app/data/gopod.db \
            -e CADDY_ADMIN_URL=unix:///run/gopod-caddy/admin.sock \
            -v "${DETECTED_SOCKET}:/run/podman/podman.sock:ro,z" \
            -v "${GOPOD_DATA}/data:/app/data:Z" \
            -v "${GOPOD_DATA}/caddy-admin:/run/gopod-caddy:z" \
            --security-opt label=disable \
            "${GOPOD_IMAGE}"

        podman run -d --name gopod-caddy \
            --restart=always \
            --network=gopod-internal,gopod-net \
            -p "${CADDY_HTTP_PORT}:80" \
            -p "${CADDY_HTTPS_PORT}:443" \
            -e CADDY_INGRESS_NETWORK=gopod-net \
            -v "${GOPOD_CONFIG}/caddy/Caddyfile:/etc/caddy/Caddyfile:Z" \
            -v "${GOPOD_DATA}/caddy/data:/data:Z" \
            -v "${GOPOD_DATA}/caddy/config:/config:Z" \
            -v "${GOPOD_DATA}/logs:/var/log/caddy:z" \
            -v "${GOPOD_DATA}/caddy-admin:/run/gopod-caddy:z" \
            "${CADDY_IMAGE}"
    fi
}

task_orchestrate_services() {
    task_generate_manifests
    case "$RUN_MODE" in
        quadlet-pod|quadlet-standalone)
            if ! start_quadlet; then
                RUN_MODE="container"
                start_compose
            fi
            ;;
        *)
            start_compose
            ;;
    esac
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

task_install_cli_wrapper() {
    create_cli_wrapper "$CURRENT_USER" "$USER_UID" "$USER_HOME" "$CONFIG_DIR" "$DATA_DIR" "$GOPOD_PORT" "$CADDY_HTTP_PORT"
}
run_task "Installing GOPOD CLI management wrapper (/usr/local/bin/gopod)" task_install_cli_wrapper

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
echo -e " CLI Helper    : ${CYAN}${BOLD}/usr/local/bin/gopod${NC} ${GREEN}(Auto-configured, ready to use)${NC}"
echo "──────────────────────────────────────────────────────────────"
echo -e "${BOLD}Management Cheat Sheet (Run directly from any terminal):${NC}"
echo -e "  ${CYAN}gopod status${NC}           Check health, podman socket, & container state"
echo -e "  ${CYAN}gopod ps${NC}               List running GOPOD containers"
echo -e "  ${CYAN}gopod logs -f${NC}          Stream live unified service logs"
echo -e "  ${CYAN}gopod restart${NC}          Restart GOPOD PaaS & Caddy Ingress"
echo -e "  ${CYAN}gopod stop${NC}             Stop all GOPOD services"
echo -e "  ${CYAN}gopod start${NC}            Start GOPOD services"
echo -e "  ${CYAN}gopod shell [caddy]${NC}    Open interactive shell inside container"
echo -e "  ${CYAN}gopod update${NC}           Pull latest container images & restart stack"
echo -e "  ${CYAN}gopod info${NC}             Show runtime environment & config paths"
echo -e "  ${CYAN}gopod podman <args>${NC}   Run any podman command in rootless context"
echo -e "\n${BLUE}Open your browser at ${BOLD}http://${SERVER_IP}:${GOPOD_PORT}${NC}${BLUE} to begin the setup wizard.${NC}\n"
