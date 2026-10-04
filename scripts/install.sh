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
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BLUE}${BOLD}"
echo "=========================================================="
echo "          GOPOD — Rootless Podman PaaS Installer          "
echo "=========================================================="
echo -e "${NC}"

CURRENT_USER=$(id -un)
USER_HOME=$(eval echo "~${CURRENT_USER}")

if [ "$CURRENT_USER" = "root" ]; then
    echo -e "${YELLOW}⚠️  Warning: Running as root. For maximum container security and cgroup isolation,"
    echo -e "   GOPOD recommends running as an unprivileged regular user.${NC}"
fi

# 1. Check or install Podman
echo -e "\n${BOLD}[1/5] Checking Podman installation...${NC}"
if ! command -v podman &> /dev/null; then
    echo "Installing Podman..."
    if command -v apt-get &> /dev/null; then
        sudo apt-get update && sudo apt-get install -y podman systemd
    elif command -v dnf &> /dev/null; then
        sudo dnf install -y podman systemd
    elif command -v pacman &> /dev/null; then
        sudo pacman -Sy --noconfirm podman systemd
    else
        echo -e "${RED}Unsupported package manager. Please install Podman manually.${NC}"
        exit 1
    fi
fi
echo -e "${GREEN}✓ Podman $(podman --version) is available.${NC}"

# 2. Allow unprivileged port binding (ports 80 & 443)
echo -e "\n${BOLD}[2/5] Configuring unprivileged port access (<1024)...${NC}"
SYSCTL_FILE="/etc/sysctl.d/99-gopod.conf"
if [ ! -f "$SYSCTL_FILE" ]; then
    echo "net.ipv4.ip_unprivileged_port_start=80" | sudo tee "$SYSCTL_FILE" > /dev/null
    sudo sysctl -p "$SYSCTL_FILE" > /dev/null 2>&1 || sudo sysctl --system > /dev/null 2>&1
    echo -e "${GREEN}✓ Unprivileged port start set to 80.${NC}"
else
    echo -e "${GREEN}✓ Unprivileged port configuration already active.${NC}"
fi

# 3. Enable user lingering and Podman socket
echo -e "\n${BOLD}[3/5] Enabling systemd user lingering and Podman socket...${NC}"
if command -v loginctl &> /dev/null && [ "$CURRENT_USER" != "root" ]; then
    sudo loginctl enable-linger "$CURRENT_USER"
    echo -e "${GREEN}✓ Lingering enabled for ${CURRENT_USER}.${NC}"
fi

systemctl --user enable --now podman.socket
echo -e "${GREEN}✓ Podman user socket is running: $(podman info --format '{{.Host.RemoteConnectionInfo.URL}}' 2>/dev/null || echo 'active')${NC}"

# 4. Prepare directories and Quadlet manifests
echo -e "\n${BOLD}[4/5] Setting up GOPOD Quadlet systemd service...${NC}"
QUADLET_DIR="${USER_HOME}/.config/containers/systemd"
GOPOD_CONFIG="${USER_HOME}/.config/gopod"
GOPOD_DATA="${USER_HOME}/.local/share/gopod"

mkdir -p "${QUADLET_DIR}"
mkdir -p "${GOPOD_CONFIG}/caddy"
mkdir -p "${GOPOD_DATA}/data"
mkdir -p "${GOPOD_DATA}/caddy/data"
mkdir -p "${GOPOD_DATA}/logs"

# Copy or generate Quadlet units
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

[Install]
WantedBy=default.target
EOF

cat <<'EOF' > "${QUADLET_DIR}/gopod-caddy.container"
[Unit]
Description=GOPOD Caddy Ingress Reverse Proxy
PartOf=gopod-pod.service
After=gopod-pod.service

[Container]
ContainerName=gopod-caddy
Pod=gopod.pod
Image=docker.io/library/caddy:alpine
AutoUpdate=registry
Volume=%h/.config/gopod/caddy:/etc/caddy:Z
Volume=%h/.local/share/gopod/caddy/data:/data:Z
Volume=%h/.local/share/gopod/logs:/var/log/caddy:Z

[Service]
Restart=always
RestartSec=3s

[Install]
WantedBy=default.target
EOF

cat <<'EOF' > "${QUADLET_DIR}/gopod-server.container"
[Unit]
Description=GOPOD PaaS Control Plane (Go Single Binary + SQLite)
PartOf=gopod-pod.service
After=gopod-pod.service

[Container]
ContainerName=gopod-server
Pod=gopod.pod
Image=ghcr.io/ijon6k/gopod:dev
AutoUpdate=registry
Environment=PORT=8085
Environment=PODMAN_SOCKET=/run/podman/podman.sock
Environment=GOPOD_DB_PATH=/app/data/gopod.db
Environment=CADDY_ADMIN_URL=http://127.0.0.1:2019
Volume=%t/podman/podman.sock:/run/podman/podman.sock:Z
Volume=%h/.local/share/gopod/data:/app/data:Z

[Service]
Restart=always
RestartSec=3s

[Install]
WantedBy=default.target
EOF

echo -e "${GREEN}✓ Quadlet unit files written to ${QUADLET_DIR}.${NC}"

# 5. Reload systemd daemon and activate service
echo -e "\n${BOLD}[5/5] Activating GOPOD via systemd Quadlet...${NC}"
systemctl --user daemon-reload
systemctl --user start gopod-pod.service || true

SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")

echo -e "\n${GREEN}${BOLD}=========================================================="
echo "          GOPOD Installation Finished Successfully!       "
echo "==========================================================${NC}"
echo -e "Dashboard URL : ${BOLD}http://${SERVER_IP}:8085${NC} (or http://${SERVER_IP} via Caddy)"
echo -e "Service Status: ${BOLD}systemctl --user status gopod-pod.service${NC}"
echo -e "Podman Pod    : ${BOLD}podman pod ps${NC}"
echo -e "Database Path : ${BOLD}${GOPOD_DATA}/data/gopod.db${NC}"
echo -e "${BLUE}Enjoy building with rootless Podman!${NC}\n"
