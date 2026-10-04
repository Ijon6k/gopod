# GOPOD Development Instructions & Rules

This repository follows strict design, architecture, and code quality standards.

## Workspace Rules
- [UI & Table Design Standards](file:///.agents/rules/ui-and-table-standards.md) (`.agents/rules/ui-and-table-standards.md`)
  - **Zero Hardcoded Colors**: Always use semantic design tokens from `frontend/src/app.css` (`var(--bg-table-row)`, `var(--bg-table-row-alt)`, `var(--bg-table-header)`, `var(--accent-muted)`, etc.). Never use low-opacity fractional classes (e.g. `/20`) for row differentiation.
  - **Solid Opaque Table Headers (No Blur)**: Never use `backdrop-blur` on table headers. Use solid `bg-[var(--bg-table-header)]` with `border-b border-[var(--border)]`.
  - **Natural Table Height**: Do not use inner `max-h-[...]` or inner vertical scrollbars inside data tables. Table height must be content-driven while column headers remain `sticky top-0 z-20`.
  - **Desktop Sticky Preservation**: Use `overflow-x-auto md:overflow-x-visible` on table containers so outer desktop scrolling preserves sticky header pinning.
  - **Typography & Touch Targets**: Primary table data rows must use 13px–14px typography. Row hover actions must be at least 28x28px (`p-1.5`) with 14px+ icons.
  - **Language**: All UI copy, labels, tooltips, dialogs, and badges must strictly be in **English**.
- [GOPOD Architecture & Real-World Standards](file:///.agents/rules/gopod-architecture-and-real-world-parity.md) (`.agents/rules/gopod-architecture-and-real-world-parity.md`)
  - **Zero Gimmicks**: Features and UI must map 1:1 to real Linux, Podman rootless socket, cgroups v2, systemd, and Caddy capabilities.
  - **First-Class Quadlet**: Full support for systemd-managed `.container` and `.pod` declarative services alongside Compose and Kube YAML.
  - **Centralized Credentials**: Reusable SSH deploy keys, PATs, and container registry credentials.
  - **Caddy Ingress Parity**: Real reverse proxy features (Automatic Let's Encrypt HTTPS, websockets, custom headers/CORS, path routing, middlewares).
  - **PaaS Lifecycle**: Deploy triggers, webhooks, rollback, streaming logs, and container exec terminal.
