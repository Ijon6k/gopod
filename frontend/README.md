# GoPod Frontend

SvelteKit 2 / Svelte 5 (runes), TypeScript, Tailwind CSS 4. Built with Bun and
shipped as a static SPA embedded into the Go binary (`//go:embed` in
`backend/embed.go`).

## Commands

```bash
bun install
bun run dev     # dev server
bun run check   # typecheck (svelte-check)
bun run lint    # prettier + eslint
bun run build   # production build
```

Design tokens, table/UI rules, and the contribution flow live in
[../CONTRIBUTING.md](../CONTRIBUTING.md).

The entire project is licensed under the GNU AGPL-3.0; see
[../LICENSE](../LICENSE).
