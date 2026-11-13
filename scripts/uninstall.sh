#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GOPOD — Modern Rootless Podman PaaS Uninstaller Script
# Completely tears down GOPOD services, containers, networks, quadlets, and data.
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
CYAN='\033[0;36m'
NC='\033[0m'

PURGE_DATA=false
NON_INTERACTIVE=false
REMOVE_USER=false
SERVICE_USER="gopod"

CURRENT_USER=$(id -un)
USER_UID=$(id -u)
USER_HOME=$(eval echo "~${CURRENT_USER}")

show_help() {
    cat <<EOF
${BOLD}GOPOD Uninstaller${NC} — Cleanly remove GOPOD PaaS services and artifacts

${BOLD}Usage:${NC}
  ./scripts/uninstall.sh [OPTIONS]
  gopod uninstall [OPTIONS]

${BOLD}Options:${NC}
  -p, --purge         Permanently delete persistent data, databases, and logs
  -y, --yes           Non-interactive mode: confirm removal without prompts
  --remove-user       Remove dedicated 'gopod' system user (requires root/sudo)
  --user=<NAME>       Target specific service user (Default: gopod)
  -h, --help          Show this help dialog

${BOLD}Examples:${NC}
  ./scripts/uninstall.sh                # Stop services, remove containers/quadlets, keep data
  ./scripts/uninstall.sh --purge -y     # Complete non-interactive purge of everything
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -p|--purge)
            PURGE_DATA=true
            ;;
        -y|--yes|--non-interactive)
            NON_INTERACTIVE=true
            ;;
        --remove-user)
            REMOVE_USER=true
            ;;
        --user=*)
            SERVICE_USER="${1#*=}"
            ;;
        --user)
            SERVICE_USER="$2"; shift
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

echo -e "\n${RED}${BOLD}=============================================================="
echo "          GOPOD Complete Uninstallation & Teardown            "
echo "=============================================================="
echo -e "${NC}"
echo -e " Target Mode  : ${BOLD}Full Teardown${NC}"
echo -e " Purge Data   : $([ "$PURGE_DATA" = true ] && echo "${RED}${BOLD}YES (All databases and certs will be deleted)${NC}" || echo "${GREEN}NO (Keep persistent storage)${NC}")"
echo -e " User Context : ${BOLD}${CURRENT_USER}${NC} (UID: ${USER_UID})"
echo "──────────────────────────────────────────────────────────────"

if [ "$NON_INTERACTIVE" = false ] && ([ -t 0 ] || [ -e /dev/tty ]); then
    TTY_DEV="/dev/tty"
    exec 3< "$TTY_DEV" 2>/dev/null || exec 3<&0
    read -r -u 3 -p "Are you sure you want to uninstall GOPOD? [y/N]: " CONFIRM_UNINSTALL
    exec 3<&-
    case "$CONFIRM_UNINSTALL" in
        y|Y|yes|YES)
            ;;
        *)
            echo -e "\n${YELLOW}Uninstallation aborted by user.${NC}"
            exit 0
            ;;
    esac
fi

# Helper to run commands in rootless context if needed
run_podman() {
    if [ "$USER_UID" -eq 0 ] && [ "$CURRENT_USER" != "$SERVICE_USER" ] && id -u "$SERVICE_USER" &>/dev/null; then
        local svc_uid
        svc_uid=$(id -u "$SERVICE_USER")
        local run_dir="/run/user/${svc_uid}"
        local sock="${run_dir}/podman/podman.sock"
        su - "$SERVICE_USER" -c "export XDG_RUNTIME_DIR='${run_dir}' DBUS_SESSION_BUS_ADDRESS='unix:path=${run_dir}/bus' DOCKER_HOST='unix://${sock}'; podman $*" 2>/dev/null || true
    else
        podman "$@" 2>/dev/null || true
    fi
}

echo -e "\n${CYAN}==> [1/6] Stopping systemd Quadlet & user services...${NC}"
# Stop systemd user units for current user
if command -v systemctl &>/dev/null; then
    if [ "$USER_UID" -ne 0 ]; then
        systemctl --user stop gopod-pod.service gopod-server.service gopod-caddy.service 2>/dev/null || true
        systemctl --user disable gopod-pod.service gopod-server.service gopod-caddy.service 2>/dev/null || true
    else
        systemctl stop gopod-pod.service gopod-server.service gopod-caddy.service 2>/dev/null || true
        systemctl disable gopod-pod.service gopod-server.service gopod-caddy.service 2>/dev/null || true
    fi

    # Also check if dedicated service user has active units
    if [ "$USER_UID" -eq 0 ] && id -u "$SERVICE_USER" &>/dev/null; then
        local svc_uid
        svc_uid=$(id -u "$SERVICE_USER")
        run_podman systemctl --user stop gopod-pod.service gopod-server.service gopod-caddy.service 2>/dev/null || true
        run_podman systemctl --user disable gopod-pod.service gopod-server.service gopod-caddy.service 2>/dev/null || true
    fi
fi
echo -e "  ${GREEN}✓${NC} Systemd services stopped."

echo -e "\n${CYAN}==> [2/6] Stopping and removing containers and pods...${NC}"
# Stop and force remove core containers
run_podman stop -t 2 gopod gopod-caddy 2>/dev/null || true
run_podman rm -f gopod gopod-caddy 2>/dev/null || true

# Stop and force remove gopod pod if present
run_podman pod stop -t 2 gopod 2>/dev/null || true
run_podman pod rm -f gopod 2>/dev/null || true
echo -e "  ${GREEN}✓${NC} Containers and pods removed."

echo -e "\n${CYAN}==> [3/6] Cleaning up isolated bridge networks...${NC}"
run_podman network rm -f gopod-internal 2>/dev/null || true
run_podman network rm -f gopod-net 2>/dev/null || true
echo -e "  ${GREEN}✓${NC} Networks cleaned up."

echo -e "\n${CYAN}==> [4/6] Removing Quadlet manifests and reloading systemd...${NC}"
# Remove user quadlets
rm -f "${USER_HOME}/.config/containers/systemd/gopod"* 2>/dev/null || true
# Remove root quadlets if run as root
if [ "$USER_UID" -eq 0 ]; then
    rm -f /etc/containers/systemd/gopod* 2>/dev/null || true
    rm -f /root/.config/containers/systemd/gopod* 2>/dev/null || true
fi

# Remove quadlets for service user if exists
if id -u "$SERVICE_USER" &>/dev/null; then
    local target_home
    target_home=$(eval echo "~${SERVICE_USER}")
    rm -f "${target_home}/.config/containers/systemd/gopod"* 2>/dev/null || true
fi

if command -v systemctl &>/dev/null; then
    if [ "$USER_UID" -ne 0 ]; then
        systemctl --user daemon-reload 2>/dev/null || true
    else
        systemctl daemon-reload 2>/dev/null || true
    fi
fi
echo -e "  ${GREEN}✓${NC} Quadlet manifests purged."

echo -e "\n${CYAN}==> [5/6] Removing CLI wrappers and system sysctl configurations...${NC}"
rm -f /usr/local/bin/gopod /usr/bin/gopod "${USER_HOME}/.local/bin/gopod" 2>/dev/null || true
if [ "$USER_UID" -eq 0 ]; then
    rm -f /etc/sysctl.d/99-gopod.conf 2>/dev/null || true
fi
echo -e "  ${GREEN}✓${NC} CLI wrappers removed."

echo -e "\n${CYAN}==> [6/6] Handling storage directories and configurations...${NC}"
# Remove configuration dir
rm -rf "${USER_HOME}/.config/gopod" 2>/dev/null || true

if [ "$PURGE_DATA" = true ]; then
    echo -e "  ${YELLOW}Purging persistent data directories...${NC}"
    rm -rf "${USER_HOME}/.local/share/gopod" 2>/dev/null || true
    if id -u "$SERVICE_USER" &>/dev/null; then
        local target_home
        target_home=$(eval echo "~${SERVICE_USER}")
        rm -rf "${target_home}/.local/share/gopod" "${target_home}/.config/gopod" 2>/dev/null || true
    fi
    echo -e "  ${GREEN}✓${NC} Persistent data purged."
else
    echo -e "  ${GREEN}✓${NC} Preserved persistent data at ${BOLD}${USER_HOME}/.local/share/gopod${NC}"
fi

# Disable linger and remove user if requested
if [ "$USER_UID" -eq 0 ] && id -u "$SERVICE_USER" &>/dev/null; then
    if command -v loginctl &>/dev/null; then
        loginctl disable-linger "$SERVICE_USER" 2>/dev/null || true
    fi
    if [ "$REMOVE_USER" = true ]; then
        echo -e "\n${CYAN}==> Removing dedicated service user '${SERVICE_USER}'...${NC}"
        pkill -u "$SERVICE_USER" 2>/dev/null || true
        userdel -r "$SERVICE_USER" 2>/dev/null || userdel "$SERVICE_USER" 2>/dev/null || true
        echo -e "  ${GREEN}✓${NC} User '${SERVICE_USER}' removed."
    fi
fi

echo -e "\n${GREEN}${BOLD}=============================================================="
echo "          GOPOD Has Been Successfully Uninstalled!            "
echo "==============================================================${NC}\n"
