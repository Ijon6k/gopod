# Contributing to GOPOD

Thank you for your interest in contributing to **GOPOD**! GOPOD is a lightweight, self-hosted Platform-as-a-Service (PaaS) built with **Podman Native** (rootless socket, pods, and Quadlet systemd service units), Go, Caddy ingress proxy, and SvelteKit.

---

## 🏗️ Architecture Overview

- **Backend (`/backend`)**:
  - Written in Go (1.26+).
  - Uses modern SQLite (`modernc.org/sqlite`) for zero-dependency local state persistence.
  - Interacts directly with the Podman daemon via `/run/podman/podman.sock` (HTTP REST API / libpod).
  - Communicates with Caddy's dynamic admin API (`http://caddy:2019`).
- **Frontend (`/frontend`)**:
  - Written in SvelteKit 2 + Svelte 5 (Runes mode) + TypeScript + Tailwind CSS.
  - Design system governed by semantic CSS variables (`frontend/src/app.css`).
  - Managed using **Bun** (`bun run dev`, `bun run build`).
- **Reverse Proxy (`/caddy`)**:
  - Caddy 2 Alpine container for zero-touch Let's Encrypt TLS and reverse proxying to `gopod-net`.

---

## 🛠️ Development Setup

### Prerequisites
- [Podman](https://podman.io/) (v4.5+ or v5.x recommended)
- [Go](https://golang.org/) (v1.23+)
- [Bun](https://bun.sh/) (v1.1+)
- Linux environment or WSL2 with systemd and rootless Podman socket enabled:
  ```bash
  systemctl --user enable --now podman.socket
  ```

### Running with Compose (Development)
You can bring up the entire dev stack (Backend + Frontend + Caddy proxy) using:
```bash
podman compose -f compose.dev.yml up -d --build
```
Or run the services individually:

#### 1. Backend
```bash
cd backend
go run ./cmd/server
```

#### 2. Frontend
```bash
cd frontend
bun install
bun run dev
```

---

## 🎨 UI & Frontend Design Rules

Contributions to the frontend must follow the **GOPOD UI & Table Design Standards**:

1. **Zero Hardcoded Colors**: Always use semantic design tokens from `frontend/src/app.css` (`var(--bg-table-row)`, `var(--bg-table-row-alt)`, `var(--bg-table-header)`, `var(--accent-muted)`, `var(--text-primary)`, etc.). Never use low-opacity fractional classes (e.g., `/20`) for row differentiation.
2. **Solid Opaque Table Headers**: Never use `backdrop-blur` on table headers. Use solid `bg-[var(--bg-table-header)]` with `border-b border-[var(--border)]`.
3. **Natural Table Height**: Do not use inner `max-h-[...]` or inner vertical scrollbars inside data tables. Table height must be content-driven while column headers remain `sticky top-0 z-20`.
4. **Desktop Sticky Preservation**: Use `overflow-x-auto md:overflow-x-visible` on table containers so outer desktop scrolling preserves sticky header pinning.
5. **Component Modularity**: Use centralized reusable primitives (`<Modal>`, `<ConfirmDialog>`, `<FormField>`, `<SettingCard>`, `<Button>`, `<Input>`) from `$lib/components/ui` and `$lib/components/primitives` instead of duplicating dialog shells or form labels.
6. **Language**: All UI copy, labels, tooltips, dialogs, and badges must strictly be in **English**.

---

## 🧪 Linting & Quality Checks

Before submitting a Pull Request:

```bash
# Frontend typecheck & lint
cd frontend
bun run check
bun x eslint .

# Backend test
cd ../backend
go test ./...
```

---

## 🤝 Pull Request Guidelines

1. Create a feature branch from `main`: `git checkout -b feature/my-cool-feature`.
2. Commit with descriptive, conventional commit messages: `feat(quadlet): support custom systemd install section`.
3. Verify that your changes do not break existing podman socket interactions or Caddy reverse proxy routing.
4. Submit a Pull Request targeting the `main` branch.
