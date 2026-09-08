# astro-go-blog

Personal blog rebuilt as **Astro static frontend + Go backend + admin SPA**. Content is Markdown/MDX in Git; the public site is plain static files.

The project constitution (architecture, content schema, security rules, workflow) is in [AGENTS.md](AGENTS.md).

## Quick start

Requirements: Node 22, pnpm 10, Go 1.25 (backend arrives in phase 3).

```bash
pnpm install
cp site/.env.example site/.env
pnpm dev
```

Open http://localhost:4321.

| Command | What it does |
| --- | --- |
| `pnpm dev` | Astro dev server on port 4321 (drafts visible) |
| `pnpm build` | Production build to `site/dist/` (drafts excluded) |
| `pnpm check` | Type-check Astro, TypeScript and Vue files |
| `pnpm preview` | Serve the production build locally |

## Layout

- `content/` — posts, works, pages, friends data (the single source of truth)
- `site/` — Astro project
- `api/` — Go backend (later phase)
- `admin/` — Vue admin SPA (later phase)
- `docs/decisions/` — architecture decision records
