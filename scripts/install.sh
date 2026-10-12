#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GOPOD — Modern Rootless Podman PaaS Installation Script
# https://github.com/ijon6k/gopod
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BLUE}${BOLD}"
echo "=========================================================="
echo "          GOPOD — Rootless Podman PaaS Installer          "
echo "=========================================================="
echo -e "${NC}"

CURRENT_USER=$(id -un)
USER_UID=$(id -u)
USER_HOME=$(eval echo "~${CURRENT_USER}")

if [ "$CURRENT_USER" = "root" ]; then
    echo -e "${YELLOW}⚠️  Notice: Running as root. For maximum container security and cgroup isolation,"
    echo -e "   GOPOD recommends running as a regular unprivileged user (e.g. 'ubuntu', 'rocky', 'debian').${NC}\n"
fi

# Parse CLI options
RUN_MODE=""
for arg in "$@"; do
    case "$arg" in
        --mode=quadlet|--quadlet) RUN_MODE="quadlet" ;;
        --mode=compose|--compose) RUN_MODE="compose" ;;
        --help|-h)
            echo "Usage: ./install.sh [OPTIONS]"
            echo "Options:"
            echo "  --mode=quadlet    Install and run as native systemd Quadlet service (default)"
            echo "  --mode=compose    Install and run via Podman Compose"
            echo "  --help, -h        Show this help message"
            exit 0
            ;;
    esac
done

# ─────────────────────────────────────────────────────────────────────────────
# [1/7] Linux Distro & SELinux Detection
# ─────────────────────────────────────────────────────────────────────────────
echo -e "${BOLD}[1/7] Detecting Linux distribution and security policies...${NC}"

DISTRO_ID="unknown"
DISTRO_NAME="Unknown Linux"
DISTRO_LIKE=""
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO_ID="${ID:-unknown}"
    DISTRO_NAME="${NAME:-Linux}"
    DISTRO_LIKE="${ID_LIKE:-}"
fi
echo -e "${GREEN}✓ OS Detected: ${DISTRO_NAME} (${DISTRO_ID})${NC}"

# Detect SELinux (Common in RHEL, Fedora, CentOS, Rocky Linux, AlmaLinux)
SELINUX_STATUS="disabled"
if command -v getenforce &> /dev/null; then
    SELINUX_STATUS=$(getenforce 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo "disabled")
elif [ -f /etc/selinux/config ]; then
    SELINUX_STATUS=$(grep -E '^SELINUX=' /etc/selinux/config 2>/dev/null | cut -d= -f2 | tr '[:upper:]' '[:lower:]' || echo "disabled")
fi

if [ "$SELINUX_STATUS" = "enforcing" ] || [ "$SELINUX_STATUS" = "permissive" ]; then
    echo -e "${CYAN}✓ SELinux detected: ${SELINUX_STATUS}${NC}"
    echo "  Applying container security policies for rootless Podman..."
    if command -v setsebool &> /dev/null; then
        sudo setsebool -P container_manage_cgroup 1 2>/dev/null || true
    fi
else
    echo -e "${GREEN}✓ SELinux status: Not enforcing / inactive.${NC}"
fi

# ─────────────────────────────────────────────────────────────────────────────
# [2/7] Package Installation & Dependencies
# ─────────────────────────────────────────────────────────────────────────────
echo -e "\n${BOLD}[2/7] Checking Podman, systemd, and subuid dependencies...${NC}"

if ! command -v podman &> /dev/null; then
    echo "Installing Podman and system dependencies..."
    if command -v apt-get &> /dev/null; then
        sudo apt-get update && sudo apt-get install -y podman systemd uidmap curl jq
    elif command -v dnf &> /dev/null; then
        sudo dnf install -y podman systemd shadow-utils curl jq
    elif command -v pacman &> /dev/null; then
        sudo pacman -Sy --noconfirm podman systemd shadow curl jq
    elif command -v zypper &> /dev/null; then
        sudo zypper install -y podman systemd shadow curl jq
    else
        echo -e "${RED}Unsupported package manager. Please install Podman manually.${NC}"
        exit 1
    fi
fi
echo -e "${GREEN}✓ Podman $(podman --version) is ready.${NC}"

# Ensure subuid and subgid ranges are present for rootless mapping
if [ "$CURRENT_USER" != "root" ]; then
    if ! grep -q "^${CURRENT_USER}:" /etc/subuid 2>/dev/null; then
        echo "Configuring subuid allocation for ${CURRENT_USER}..."
        sudo usermod --add-subuids 100000-165535 "$CURRENT_USER" 2>/dev/null || true
    fi
    if ! grep -q "^${CURRENT_USER}:" /etc/subgid 2>/dev/null; then
        echo "Configuring subgid allocation for ${CURRENT_USER}..."
        sudo usermod --add-subgids 100000-165535 "$CURRENT_USER" 2>/dev/null || true
    fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# [3/7] Allow Unprivileged Binding to Ports 80 and 443
# ─────────────────────────────────────────────────────────────────────────────
echo -e "\n${BOLD}[3/7] Configuring unprivileged port binding (ports 80 & 443)...${NC}"
CURRENT_START_PORT=$(cat /proc/sys/net/ipv4/ip_unprivileged_port_start 2>/dev/null || echo "1024")

if [ "$CURRENT_START_PORT" -gt 80 ]; then
    SYSCTL_FILE="/etc/sysctl.d/99-gopod.conf"
    echo "net.ipv4.ip_unprivileged_port_start=80" | sudo tee "$SYSCTL_FILE" > /dev/null
    sudo sysctl -p "$SYSCTL_FILE" > /dev/null 2>&1 || sudo sysctl --system > /dev/null 2>&1
    echo -e "${GREEN}✓ Unprivileged port start successfully lowered to 80.${NC}"
else
    echo -e "${GREEN}✓ Unprivileged port start is already <= 80 (${CURRENT_START_PORT}).${NC}"
fi

# ─────────────────────────────────────────────────────────────────────────────
# [4/7] Systemd Lingering & User Socket Activation
# ─────────────────────────────────────────────────────────────────────────────
echo -e "\n${BOLD}[4/7] Enabling systemd user lingering and Podman socket...${NC}"
if command -v loginctl &> /dev/null && [ "$CURRENT_USER" != "root" ]; then
    sudo loginctl enable-linger "$CURRENT_USER" 2>/dev/null || true
    echo -e "${GREEN}✓ Systemd lingering enabled for ${CURRENT_USER} (services persist across logout).${NC}"
fi

systemctl --user enable --now podman.socket 2>/dev/null || true
SOCKET_URL=$(podman info --format '{{.Host.RemoteConnectionInfo.URL}}' 2>/dev/null || echo "/run/user/${USER_UID}/podman/podman.sock")
echo -e "${GREEN}✓ Podman rootless socket is active: ${SOCKET_URL}${NC}"

# Enable podman-restart.service so containers with 'restart: always' boot reliably
sudo systemctl enable --now podman-restart.service 2>/dev/null || systemctl --user enable --now podman-restart.service 2>/dev/null || true

# ─────────────────────────────────────────────────────────────────────────────
# [5/7] Pre-create Persistent Directories & Permissions (Prevent Root Takeover)
# ─────────────────────────────────────────────────────────────────────────────
echo -e "\n${BOLD}[5/7] Initializing storage directories and file permissions...${NC}"
GOPOD_CONFIG="${USER_HOME}/.config/gopod"
GOPOD_DATA="${USER_HOME}/.local/share/gopod"
QUADLET_DIR="${USER_HOME}/.config/containers/systemd"

# Ensure all mount targets are created BEFORE containers launch
mkdir -p "${GOPOD_CONFIG}/caddy"
mkdir -p "${GOPOD_DATA}/data"
mkdir -p "${GOPOD_DATA}/caddy/data"
mkdir -p "${GOPOD_DATA}/caddy/config"
mkdir -p "${GOPOD_DATA}/logs"
mkdir -p "${GOPOD_DATA}/deployments"
mkdir -p "${GOPOD_DATA}/keys"
mkdir -p "${QUADLET_DIR}"

# Crucial: Pre-create access.log as a regular file so Podman does NOT create it as a root-owned directory!
if [ ! -f "${GOPOD_DATA}/logs/access.log" ]; then
    touch "${GOPOD_DATA}/logs/access.log"
fi

# Pre-create default Caddyfile if not already present
if [ ! -f "${GOPOD_CONFIG}/caddy/Caddyfile" ]; then
    cat <<'EOF' > "${GOPOD_CONFIG}/caddy/Caddyfile"
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
    reverse_proxy 127.0.0.1:8085
}
EOF
fi

# Set proper ownership and permissions
chmod 750 "${GOPOD_DATA}" "${GOPOD_CONFIG}"
chmod 700 "${GOPOD_DATA}/keys"
chmod 644 "${GOPOD_DATA}/logs/access.log"

# Apply SELinux container labels if enforcing
if [ "$SELINUX_STATUS" = "enforcing" ] || [ "$SELINUX_STATUS" = "permissive" ]; then
    if command -v chcon &> /dev/null; then
        chcon -Rt container_file_t "${GOPOD_DATA}" "${GOPOD_CONFIG}" 2>/dev/null || true
        echo -e "${GREEN}✓ SELinux container_file_t context applied.${NC}"
    fi
fi
echo -e "${GREEN}✓ Persistent directories and permissions verified.${NC}"

# ─────────────────────────────────────────────────────────────────────────────
# [6/7] Select & Generate Manifests (Quadlet vs Compose)
# ─────────────────────────────────────────────────────────────────────────────
echo -e "\n${BOLD}[6/7] Configuring deployment mode...${NC}"

GOPOD_IMAGE="${GOPOD_IMAGE:-ghcr.io/ijon6k/gopod:latest}"

# Interactive prompt if not specified via CLI flag
if [ -z "$RUN_MODE" ]; then
    if [ -t 0 ]; then
        echo -e "Choose how you want to run GOPOD on this server:"
        echo -e "  ${BOLD}1)${NC} ${GREEN}Systemd Quadlet${NC} (Recommended — Native systemd user units, auto-update, auto-restart on boot)"
        echo -e "  ${BOLD}2)${NC} ${BLUE}Podman Compose${NC} (Standard compose file with restart: always)"
        read -r -p "Select deployment mode [1/2] (default: 1): " USER_CHOICE
        case "$USER_CHOICE" in
            2|compose|c) RUN_MODE="compose" ;;
            *) RUN_MODE="quadlet" ;;
        esac
    else
        # Non-interactive default: Quadlet if systemd user session is available, else Compose
        if command -v systemctl &> /dev/null && [ -d /run/systemd/system ]; then
            RUN_MODE="quadlet"
        else
            RUN_MODE="compose"
        fi
    fi
fi

echo -e "Selected deployment mode: ${CYAN}${BOLD}${RUN_MODE}${NC}"

# Always write production compose.yml
cat <<EOF > "${GOPOD_CONFIG}/compose.yml"
services:
  gopod:
    image: ${GOPOD_IMAGE}
    container_name: gopod
    restart: always
    ports:
      - "\${PORT:-8085}:8085"
    environment:
      - PORT=8085
      - PODMAN_SOCKET=/run/podman/podman.sock
      - GOPOD_DB_PATH=/app/data/gopod.db
      - CADDY_ADMIN_URL=http://caddy:2019
    volumes:
      - "\${PODMAN_SOCKET:-/run/user/${USER_UID}/podman/podman.sock}:/run/podman/podman.sock:ro,z"
      - "${GOPOD_DATA}/data:/app/data:Z"
      - "${GOPOD_DATA}/logs:/var/log/caddy:Z"
    networks:
      - gopod-net
    security_opt:
      - label=disable

  caddy:
    image: docker.io/library/caddy:alpine
    container_name: gopod-caddy
    restart: always
    ports:
      - "80:80"
      - "443:443"
      - "127.0.0.1:2019:2019"
    environment:
      - CADDY_INGRESS_NETWORK=gopod-net
    volumes:
      - "${GOPOD_CONFIG}/caddy/Caddyfile:/etc/caddy/Caddyfile:Z"
      - "${GOPOD_DATA}/caddy/data:/data:Z"
      - "${GOPOD_DATA}/caddy/config:/config:Z"
      - "${GOPOD_DATA}/logs:/var/log/caddy:Z"
    networks:
      - gopod-net

networks:
  gopod-net:
    name: gopod-net
    driver: bridge
EOF

# Always write systemd Quadlet files
cat <<'EOF' > "${QUADLET_DIR}/gopod-net.network"
[Network]
NetworkName=gopod-net
Subnet=10.89.0.0/16
Gateway=10.89.0.1
Internal=false
EOF

cat <<'EOF' > "${QUADLET_DIR}/gopod.pod"
[Unit]
Description=GOPOD Core System Pod (Caddy Ingress & Control Plane)
After=network-online.target
Wants=network-online.target

[Pod]
PodName=gopod
Network=gopod-net.network
PublishPort=80:80
PublishPort=443:443
PublishPort=8085:8085

[Install]
WantedBy=default.target
EOF

cat <<EOF > "${QUADLET_DIR}/gopod-caddy.container"
[Unit]
Description=GOPOD Caddy Ingress Reverse Proxy
PartOf=gopod-pod.service
After=gopod-pod.service

[Container]
ContainerName=gopod-caddy
Pod=gopod.pod
Image=docker.io/library/caddy:alpine
AutoUpdate=registry
Volume=${GOPOD_CONFIG}/caddy/Caddyfile:/etc/caddy/Caddyfile:Z
Volume=${GOPOD_DATA}/caddy/data:/data:Z
Volume=${GOPOD_DATA}/caddy/config:/config:Z
Volume=${GOPOD_DATA}/logs:/var/log/caddy:Z

[Service]
Restart=always
RestartSec=3s

[Install]
WantedBy=default.target
EOF

cat <<EOF > "${QUADLET_DIR}/gopod-server.container"
[Unit]
Description=GOPOD PaaS Control Plane (Go Single Binary + SQLite)
PartOf=gopod-pod.service
After=gopod-pod.service

[Container]
ContainerName=gopod-server
Pod=gopod.pod
Image=${GOPOD_IMAGE}
AutoUpdate=registry
Environment=PORT=8085
Environment=PODMAN_SOCKET=/run/podman/podman.sock
Environment=GOPOD_DB_PATH=/app/data/gopod.db
Environment=CADDY_ADMIN_URL=http://127.0.0.1:2019
Volume=%t/podman/podman.sock:/run/podman/podman.sock:Z
Volume=${GOPOD_DATA}/data:/app/data:Z
Volume=${GOPOD_DATA}/logs:/var/log/caddy:Z

[Service]
Restart=always
RestartSec=3s

[Install]
WantedBy=default.target
EOF

echo -e "${GREEN}✓ Production manifests generated with unified storage paths.${NC}"

# ─────────────────────────────────────────────────────────────────────────────
# [7/7] Starting GOPOD Service
# ─────────────────────────────────────────────────────────────────────────────
echo -e "\n${BOLD}[7/7] Starting GOPOD (${RUN_MODE} mode)...${NC}"

if [ "$RUN_MODE" = "quadlet" ]; then
    if command -v systemctl &> /dev/null && [ -d /run/systemd/system ]; then
        echo "Reloading systemd user daemon..."
        systemctl --user daemon-reload
        echo "Activating gopod-pod.service..."
        systemctl --user enable --now gopod-pod.service
        sleep 2
        if systemctl --user is-active --quiet gopod-pod.service; then
            echo -e "${GREEN}✓ GOPOD is actively running via Systemd Quadlet!${NC}"
        else
            echo -e "${YELLOW}⚠️  Quadlet service starting or pending, checking containers...${NC}"
        fi
    else
        echo -e "${YELLOW}Systemd user session not available. Falling back to Podman Compose...${NC}"
        RUN_MODE="compose"
    fi
fi

if [ "$RUN_MODE" = "compose" ]; then
    echo "Starting via Podman Compose..."
    if podman compose version &> /dev/null; then
        podman compose -f "${GOPOD_CONFIG}/compose.yml" up -d
    elif command -v podman-compose &> /dev/null; then
        podman-compose -f "${GOPOD_CONFIG}/compose.yml" up -d
    else
        echo "podman compose not found, installing podman-compose via package manager..."
        sudo dnf install -y podman-compose 2>/dev/null || sudo apt-get install -y podman-compose 2>/dev/null || true
        podman compose -f "${GOPOD_CONFIG}/compose.yml" up -d 2>/dev/null || podman-compose -f "${GOPOD_CONFIG}/compose.yml" up -d
    fi
    echo -e "${GREEN}✓ GOPOD is actively running via Podman Compose!${NC}"
fi

SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")

echo -e "\n${GREEN}${BOLD}=========================================================="
echo "          GOPOD Installation Finished Successfully!       "
echo "==========================================================${NC}"
echo -e "Dashboard URL  : ${BOLD}http://${SERVER_IP}:8085${NC} (or http://${SERVER_IP} via Caddy)"
echo -e "Deployment Mode: ${CYAN}${BOLD}${RUN_MODE}${NC} (Restart: Always)"
echo -e "Compose Config : ${BOLD}${GOPOD_CONFIG}/compose.yml${NC}"
echo -e "Quadlet Units  : ${BOLD}${QUADLET_DIR}/gopod-*.container${NC}"
echo -e "Database Path  : ${BOLD}${GOPOD_DATA}/data/gopod.db${NC}"
echo -e "Access Logs    : ${BOLD}${GOPOD_DATA}/logs/access.log${NC}"
echo -e "${BLUE}Ready for plug-and-play production deployments!${NC}\n"
