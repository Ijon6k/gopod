# GOPOD Architecture & Real-World Infrastructure Standards

All feature design, backend APIs, and frontend workflows in GOPOD must strictly adhere to these real-world infrastructure principles.

## 1. Zero Gimmicks / Real Infrastructure Grounding
- Every feature, form field, and button must map directly to actual underlying Linux and container primitives:
  - **Container Engine**: Podman daemonless rootless socket (`/run/user/$UID/podman/podman.sock`).
  - **Resource Accounting**: Linux cgroups v2 controllers (`cpu.weight`, `cpu.max`, `memory.max`, `pids.max`).
  - **Service Supervision**: `systemd --user` units and timers.
  - **Reverse Proxy**: Caddyfile syntax and dynamic `/config/apps/http` API.
- Never invent fictional UI states or "magic" buttons that do not correlate with real container operations.

## 2. Workload & Deployment Typology
GOPOD supports five primary workload types:
1. **Container**: Single OCI container image or git repository with Dockerfile/Containerfile.
2. **Pod**: Group of containers sharing a network namespace and localhost IPC (Podman pod).
3. **Quadlet**: Systemd-native declarative container service (`~/.config/containers/systemd/*.container`).
4. **Compose Stack**: Multi-service declarative stack powered by `podman compose`.
5. **Kubernetes Manifest**: Declarative pods, services, and configmaps deployed via `podman play kube`.

## 3. Reverse Proxy & Domain Management (Caddy Capabilities)
Domain routing configurations must reflect real-world Caddy reverse proxy features:
- **Upstream Target**: Internal container DNS or socket (e.g. `http://service-name:3000`).
- **SSL / TLS**: Automatic HTTPS via ACME (Let's Encrypt / ZeroSSL) with auto renewal.
- **Routing Rules**: Path-based routing with strip prefix options (`/api/*` -> backend).
- **Custom Headers**: X-Forwarded-For, CORS headers, security headers (HSTS, CSP).
- **WebSockets**: Native WebSocket connection upgrades.
- **Middlewares**: Basic HTTP Authentication, rate limiting, and client IP filtering.

## 4. Centralized Credential Architecture
- Never force users to paste raw private keys or registry passwords repeatedly into service forms.
- Credentials must be stored in a centralized credential store and referenced by ID/name:
  - **SSH Keys**: Ed25519/RSA deploy keys for Git repository cloning.
  - **Git Personal Access Tokens (PAT)**: For private repository HTTPS authentication.
  - **Registry Credentials**: Docker Hub, GitHub Container Registry (ghcr.io), Quay.io, AWS ECR, and custom private registries.

## 5. Deployment Lifecycle & Observability Parity
Service detail and management interfaces must provide:
- **Deployment Controls**:
  - Deploy / Redeploy.
  - Rebuild with `--no-cache`.
  - Force Pull latest base images.
  - Automated deployment Webhooks for Git push CI triggers.
  - Rollback to previous deployment commits.
- **Process Operations**: Start, Stop, Restart, Recreate, and Signal Kill.
- **Diagnostics**: Streaming live container logs and interactive terminal session (`podman exec`).

## 6. Information Architecture
- Keep service detail views modular and vertically stacked.
- Separate concerns into dedicated sub-tabs: General, Source & Build, Deploy Settings, Domains, Monitoring, Terminal, Logs, and Advanced.
