# GOPOD Development Instructions & Rules

Engineering standards and guidelines for GOPOD development.

## Container Engine & System Standards
- Follow official standards from the [Podman Documentation](https://docs.podman.io/).
- **Rootless Operations**: All container and pod operations run against the user rootless socket (`/run/user/$UID/podman/podman.sock`).
- **Systemd & Quadlet**: Support declarative service definitions (`.container`, `.pod`, `.network`, `.volume`) managed through systemd user services.
- **Resource Limits**: Rely on Linux cgroups v2 for resource metrics, quotas, and limits.
- **Ingress & Networking**: Enforce container bridge networking and Caddy reverse proxy integration (HTTPS automation, WebSocket streaming, path routing).

## Frontend & UI Standards (Kumo UI & Design System)
- Design standards align with Cloudflare Kumo UI and modern dashboard ergonomics.
- **Design Tokens**: Always use semantic design tokens from `frontend/src/app.css` (`var(--bg-table-row)`, `var(--bg-table-row-alt)`, `var(--bg-table-header)`, `var(--accent-muted)`, etc.). Avoid hardcoded colors and fractional opacity classes for rows.
- **Solid Table Headers**: Table headers must be opaque (`bg-[var(--bg-table-header)]` with `border-b border-[var(--border)]`). Do not use `backdrop-blur`.
- **Natural Table Height**: Table height must be content-driven without inner `max-h-[...]` or nested vertical scrollbars. Headers stay `sticky top-0 z-20` on desktop outer scroll (`overflow-x-auto md:overflow-x-visible`).
- **Typography & Ergonomics**: Primary data rows use 13px–14px typography. Action buttons maintain minimum 28x28px touch targets (`p-1.5` with 14px+ icons).
- **Language**: All user-facing UI copy, labels, tooltips, dialogs, and badges must strictly be in **English**.
