# GOPOD Installation Scripts & Operational Guide

This directory contains automated, battle-tested installation scripts for deploying **GOPOD** on any modern Linux distribution using rootless Podman.

---

## Quick Start

You can install GOPOD with a single `curl` command. Choose the release channel that fits your environment:

### 1. Production Stable Release (Recommended)
Installs the latest tested production build (`ghcr.io/ijon6k/gopod:latest`) running as a rootless Podman container stack:

```bash
curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh | bash
```

### 2. Beta / Development Preview
Installs the bleeding-edge development image (`ghcr.io/ijon6k/gopod:dev`):

```bash
curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install-beta.sh | bash
```

### 3. Interactive Custom Installation Wizard
Launches the interactive setup wizard to select your preferred image tag (stable, dev, or custom git SHA), deployment architecture (Container vs Quadlet), and custom ports:

```bash
curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/custom-install.sh | bash
```

### 4. Ephemeral Device Sandbox (Zero Footprint, Auto-Destroy)
Runs a full installation trial inside an isolated virtual device container (`quay.io/podman/stable`), uses isolated internal ports `:80` & `:8085`, verifies health via CLI `curl`, and automatically destroys everything on exit:

```bash
./scripts/test-in-container.sh
```

---

## Scripts Overview

| Script | Purpose | Default Image Tag | Default Mode |
| :--- | :--- | :--- | :--- |
| [`install.sh`](file:///home/pixy/Projects/gopod/scripts/install.sh) | Universal core installer. Supports non-interactive flags, piped curl execution, and dry-run modes. | `:latest` (Stable) | `container` (Podman Compose) |
| [`install-beta.sh`](file:///home/pixy/Projects/gopod/scripts/install-beta.sh) | Convenience shortcut for installing the latest beta development build. | `:dev` (Beta) | `container` (Podman Compose) |
| [`custom-install.sh`](file:///home/pixy/Projects/gopod/scripts/custom-install.sh) | Interactive configuration wizard with full terminal prompts for custom setup. | User Choice | User Choice |
| [`test-in-container.sh`](file:///home/pixy/Projects/gopod/scripts/test-in-container.sh) | Isolated Linux device sandbox. Uses internal ports `:80` & `:8085`, verifies via CLI `curl`, auto-cleans on exit. | `:dev` (Beta) | `container` (Podman Compose) |

---

## Privilege Models: Rootless vs Rootful

GOPOD defaults to rootless execution. All services and deployed applications run inside an isolated user namespace via `/run/user/$UID/podman/podman.sock`.

### Comparison

| Dimension | Rootless Mode (Default) | Rootful Mode (Advanced) |
| :--- | :--- | :--- |
| **Security Context** | User namespace (UID 1000+) | Host root (UID 0) |
| **Host Isolation** | Strict filesystem & process isolation | Full host capabilities |
| **Port Binding (<1024)** | Configured via `net.ipv4.ip_unprivileged_port_start=80` | Native |
| **Target Workloads** | Web apps, APIs, databases, workers, reverse proxy | Kernel drivers, host WireGuard TUN/TAP, eBPF |
| **Installer Action** | Automatically provisions unprivileged `gopod` user | Skips user isolation with `--allow-root` |

### When is Rootful Required?

Rootful mode is only needed when workloads require elevated Linux kernel privileges:
- Direct host network manipulation (WireGuard interface creation, raw iptables/nftables).
- Linux kernel tracing and eBPF instrumentation (Falco, Tetragon).
- Direct raw block device partitioning or formatting.

Standard web applications (Node.js, Go, Python, PHP, Ruby, Postgres, Redis, Caddy) do not require rootful privileges and run securely under rootless mode.

---

## What the Installer Does Under the Hood

The installer runs a 7-stage automated pipeline with live visual progress bars and diagnostic logging:

```
[██████████████░░░░░░░░░░]  57%  [4/7] Activating systemd user lingering and Podman socket
```

### Stage 1: Linux Distribution & Security Policies Detection
- Identifies the host Linux OS and package manager via `/etc/os-release` (Debian, Ubuntu, Arch, Fedora, RHEL, Rocky, Alma, Alpine, openSUSE).
- Checks SELinux status (`getenforce`). If enforcing or permissive, automatically activates `container_manage_cgroup 1` to permit rootless container cgroup management.

### Stage 2: Dependencies & Subordinate UID/GID Setup
- Verifies if `podman` is installed. If missing, installs Podman, `uidmap`, `curl`, and `jq` via the native package manager.
- Verifies subordinate UID (`/etc/subuid`) and GID (`/etc/subgid`) mappings for non-root users (ensuring standard allocation `100000-165535`).

### Stage 3: Unprivileged Port Binding (Ports 80 & 443)
- Checks `/proc/sys/net/ipv4/ip_unprivileged_port_start`.
- If > 80, creates `/etc/sysctl.d/99-gopod.conf` setting `net.ipv4.ip_unprivileged_port_start=80` so Caddy reverse proxy can bind standard HTTP (80) and HTTPS (443) ports without root privileges.
- Handles containerized or WSL environments gracefully if sysctl is read-only.

### Stage 4: Systemd User Lingering & Socket Activation
- For non-root users, enables systemd lingering via `loginctl enable-linger $USER`, allowing container services to persist across logouts and start on boot.
- Starts and enables `podman.socket` on the user session (`/run/user/$UID/podman/podman.sock`).
- Enables `podman-restart.service` so containers with `restart: always` boot automatically.

### Stage 5: Persistent Storage Layout & Permissions
Pre-creates all required persistent directories before launching containers to prevent Podman from accidentally creating host paths as root-owned directories:
- `~/.local/share/gopod/data` — Embedded SQLite database (`gopod.db`)
- `~/.local/share/gopod/caddy/data` — Let's Encrypt certificates & TLS state
- `~/.local/share/gopod/caddy/config` — Caddy ingress configuration cache
- `~/.local/share/gopod/logs/access.log` — Shared Caddy access logs (initialized as a regular file)
- `~/.local/share/gopod/keys` — SSH deploy keys (hardened with `chmod 700`)
- `~/.config/gopod/caddy/Caddyfile` — Default reverse proxy configuration (preserved across runs)
- Applies SELinux context (`chcon -Rt container_file_t`) where appropriate.

### Stage 6: Container Image Pulling
- Downloads the resolved GOPOD image (`:latest`, `:dev`, or specific `:sha-...`) from GitHub Container Registry (`ghcr.io/ijon6k/gopod`).
- Downloads Caddy ingress (`docker.io/library/caddy:alpine`).
- Displays a clean visual spinner and logs download progress.

### Stage 7: Service Orchestration & Health Verification
- Generates configuration specs (`compose.yml` and Systemd Quadlet units).
- Launches services according to the chosen mode (`container` or `quadlet`).
- Performs a readiness healthcheck against the web dashboard (`http://127.0.0.1:<PORT>`) to verify successful startup before reporting completion.

---

## Deployment Architecture: Container vs Quadlet

| Feature | Container / Compose Mode (Default) | Systemd Quadlet Mode (Experimental) |
| :--- | :--- | :--- |
| **Recommendation** | **Default & Recommended** | **Advanced / Systemd Purists** |
| **How it runs** | Rootless Podman Compose stack | Native `systemd --user` units (`.container`, `.pod`) |
| **Boot Persistence** | Managed via `podman-restart.service` | Managed natively by `systemd --user` |
| **Distro Compatibility** | Works across all distros, WSL2, cloud VPS, containers | Requires functional `systemd --user` daemon bus |
| **Why Container is Default** | Quadlet requires systemd generators and active D-Bus session bus. On some VPS or non-systemd environments, user buses can be missing or locked. Container mode is rock-solid and battle-tested. | Ideal when you want pure `systemctl --user status gopod-server` and declarative unit files under `~/.config/containers/systemd/`. |

---

## Command-Line Parameters Reference

`install.sh` accepts flexible CLI flags for automated or custom deployments:

```text
Usage: ./install.sh [OPTIONS]

Deployment Mode:
  --mode=container      Install as standard Podman Compose container stack (Default)
  --mode=quadlet        Install as native systemd Quadlet user units (Experimental)
  --container           Alias for --mode=container
  --quadlet             Alias for --mode=quadlet

Release Channels & Tags:
  --channel=latest      Install production stable release (:latest) [Default]
  --channel=dev         Install development preview release (:dev)
  --beta, --dev         Alias for --channel=dev
  --tag=<TAG_OR_SHA>    Install a specific image tag or git SHA (e.g. --tag=sha-1a2b3c4)

Network & Ports:
  --port=<PORT>         GOPOD web dashboard port (Default: 8085)
  --http-port=<PORT>    Caddy HTTP reverse proxy port (Default: 80)
  --https-port=<PORT>   Caddy HTTPS reverse proxy port (Default: 443)

Storage Directories:
  --data-dir=<PATH>     Persistent storage path (Default: ~/.local/share/gopod)
  --config-dir=<PATH>   Configuration path (Default: ~/.config/gopod)

Security & User Context:
  --rootless            Enforce rootless execution (Default on host OS)
  --user=<USERNAME>     Specify dedicated service user for rootless mode (Default: gopod)
  --allow-root, --root  Allow running directly under root (rootful mode)

Execution Control:
  -i, --interactive     Launch interactive setup menu
  -y, --non-interactive Run non-interactively without prompting
  --verbose             Display raw real-time command output instead of clean progress spinner
  --dry-run             Simulate pre-checks without changing files or starting containers
  -h, --help            Show help documentation
```

### CLI Examples

```bash
# Test pre-checks without making any changes (Dry-Run):
./scripts/install.sh --dry-run

# Install a specific commit SHA build:
./scripts/install.sh --tag=sha-7a8b9c0

# Install on a custom dashboard port (9000):
./scripts/install.sh --port=9000 -y

# Install under custom service user:
./scripts/install.sh --user=deployer -y

# Install as Systemd Quadlet service:
./scripts/install.sh --mode=quadlet -y
```

---

## Operations & Maintenance

GOPOD installs an auto-configured CLI helper at `/usr/local/bin/gopod`. It automatically bridges rootless user sessions, runtime sockets, and compose specs so you never need to manually switch users (`su - gopod`) or export environment variables (`XDG_RUNTIME_DIR`).

### Quick CLI Management

```bash
# Check system health, rootless socket, and running containers:
gopod status

# List active GOPOD containers:
gopod ps

# Stream live unified logs (all services or specific container):
gopod logs -f
gopod logs caddy

# Restart GOPOD PaaS and Caddy Ingress:
gopod restart
gopod restart caddy

# Stop or Start GOPOD service stack:
gopod stop
gopod start

# Open interactive shell in container:
gopod shell
gopod shell caddy

# Pull latest images and recreate container stack:
gopod update

# View runtime configuration, socket path, and ports:
gopod info

# Run any Podman command directly within rootless service context:
gopod podman stats
gopod podman images
```

### Manual Podman Commands (Alternative)

```bash
# Container / Compose mode:
podman ps --filter name=gopod
podman compose -f ~/.config/gopod/compose.yml logs -f
podman compose -f ~/.config/gopod/compose.yml restart

# Quadlet mode:
systemctl --user status gopod-pod.service
journalctl --user -u gopod-pod.service -f
systemctl --user restart gopod-pod.service
```

### Uninstalling GOPOD

To completely remove GOPOD services:

```bash
# Stop and remove containers:
podman stop gopod gopod-caddy 2>/dev/null || true
podman rm gopod gopod-caddy 2>/dev/null || true

# If Quadlet was used:
systemctl --user disable --now gopod-pod.service 2>/dev/null || true
rm -f ~/.config/containers/systemd/gopod-*

# Optional: Remove data and configuration directories
# WARNING: This deletes your SQLite database and Caddy configs!
# rm -rf ~/.local/share/gopod ~/.config/gopod
```

---

## Troubleshooting

### 1. Permission Denied Binding to Ports 80 or 443
Rootless Podman requires the kernel to allow unprivileged users to bind low ports:
```bash
sudo sysctl -w net.ipv4.ip_unprivileged_port_start=80
```
To persist across reboots, ensure `/etc/sysctl.d/99-gopod.conf` contains `net.ipv4.ip_unprivileged_port_start=80`.

### 2. "Failed to connect to bus: No medium found" in Quadlet Mode
This occurs if the user session bus is not active (common when using `su -` instead of an active SSH session or WSL).
**Fix:** Switch to default **Container mode** (`--mode=container`), which talks directly to Podman and does not rely on systemd user D-Bus.

### 3. SELinux Volume Permission Denials
Ensure the volume mounts use lowercase `:z` for shared directories (like logs) and uppercase `:Z` for private directories (like `data`). The installer configures this automatically.
If needed, manually relabel:
```bash
chcon -Rt container_file_t ~/.local/share/gopod ~/.config/gopod
```
