# Contributing

## Stack

- `backend/` — Go 1.26, embedded SQLite (modernc), Podman rootless REST client, Caddy admin API client
- `frontend/` — SvelteKit 2 / Svelte 5 (runes), TypeScript, Tailwind, Bun
- `caddy/` — Caddy 2, containers join `gopod-net` for routed services

## Setup

```bash
systemctl --user enable --now podman.socket

# full dev stack
podman compose -f compose.dev.yml up -d --build

# or run halves separately
cd backend && go run ./cmd/server
cd frontend && bun install && bun run dev
```

## Required checks

```bash
cd frontend
bun run check   # typecheck
bun run lint    # prettier + eslint

cd ../backend
go test ./...
```

## Rules

- Go: thin handlers, checked errors, tests for anything touching SQLite or the Podman client.
- Frontend: use the primitives in `$lib/components/ui` and `$lib/components/primitives`. Do not hand-roll dialog shells, form fields, or labels per feature.
- Colors come only from the semantic tokens in `frontend/src/app.css`. No hardcoded hex, no `bg-black/20`-style surfaces. Table headers are solid `var(--bg-table-header)` with `border-b`, no blur; tables have no inner scroll; data rows use 13–14px type; row hover actions are at least 28x28px.
- Table containers use `overflow-x-auto md:overflow-x-visible` so sticky headers keep working on desktop.
- All user-facing copy is in English.
- Features must map to real Linux, Podman rootless socket, systemd, and Caddy capabilities. No simulated behavior.

## Commits and PRs

- Conventional commits with a scope: `feat(quadlet): ...`, `fix(ingress): ...`
- One topic per PR, branch off `main`
- Changes to the Podman client, Caddy routes, or systemd units must be exercised against a real rootless Podman instance before review — mocks alone are not evidence

## License

By contributing you agree that your work is licensed to this project under the GNU AGPL-3.0 (`AGPL-3.0-only`).
