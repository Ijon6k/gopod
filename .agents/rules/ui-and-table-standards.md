# UI & Table Design Standards

All frontend code in GOPOD must strictly adhere to these visual, layout, and architectural invariants.

## 1. Strict Design Tokens (Theme Fidelity)
- **Zero Hardcoded Colors**: Never use arbitrary hex colors (e.g. `#15181D`) or raw `rgba(...)` in component markup or styles.
- **No Low-Opacity Background Hacks**: Never use fractional opacity classes like `bg-[var(--bg-surface)]/20` or `/25` for row backgrounds or surfaces. They produce imperceptible contrast in dark mode and wash out in light mode.
- **Semantic CSS Tokens**: Always use predefined tokens from `app.css`:
  - Table base row: `bg-[var(--bg-table-row)]`
  - Table alternate row (zebra): `bg-[var(--bg-table-row-alt)]`
  - Table hover state: `hover:bg-[var(--bg-table-row-hover)]`
  - Table column header: `bg-[var(--bg-table-header)]`
  - Project/Tree grouping rows: `bg-[var(--bg-table-group)]`, `bg-[var(--bg-table-subgroup)]`
  - Muted badge/icon surfaces: `bg-[var(--accent-muted)]`
- Ensure every new token is defined with distinct, accessible contrast in both `:root` (dark) and `[data-theme="light"]` (light).

## 2. Table & Data Grid Layout Architecture
- **Solid Opaque Headers (No Blur)**: Table headers (`thead`, `th`) must NEVER use `backdrop-blur` or transparent opacities. Always apply solid `bg-[var(--bg-table-header)]` with `border-b border-[var(--border)]` so scrolled rows never bleed through.
- **Natural Content-Driven Height**: Never clamp tables with inner `max-h-[...]` or inner vertical scroll containers (`overflow-y-auto`). Tables must expand naturally to fit their content. The only vertical scrollbar should belong to the main page/shell.
- **Sticky Column Headers**: Column headers must remain pinned (`sticky top-0 z-20`) when scrolling through large datasets. On the outer table wrapper, use `overflow-x-auto md:overflow-x-visible` so desktop views do not trap sticky headers into dummy scroll containers.
- **Scannable Alternating Contrast**: Flat data tables must use clean zebra striping (`idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)]' : 'bg-[var(--bg-table-row)]'`).

## 3. Typography & Hit Target Sizing
- **Data Rows**: Standard table rows must use 13px–14px font sizing (`text-[13px]` / `text-[14px]`), never cramped 10px–11px text for primary entity names.
- **Row Action Buttons**: Hover action buttons (terminal, logs, restart, delete) must provide a minimum 28x28px hit target (`p-1.5`) with 14px+ icons and high-contrast hover feedback.

## 4. Language & Copy Invariant
- All UI copy, labels, tooltips, dialogs, and status badges must strictly be written in **English**.
