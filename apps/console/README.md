# console

The Loom web UI. A Next.js dashboard for browsing registered migrations, starting runs, and tracking progress.

## What it does

- **Dashboard** (`/`) — lists all registered migrations with candidate progress at a glance
- **Migration detail** (`/migrations/[id]`) — shows the candidate table with status filtering, progress bar, overview modal, and lets you preview or cancel candidates
- **Preview** (`/migrations/[id]/preview/[candidateId]`) — dry-run preview of what a migration would do for a specific candidate
- **Steps** (`/migrations/[id]/candidates/[candidateId]/steps`) — live step progress for a running or completed candidate, including PR links and step status
- **Metrics** (`/metrics`) — migration metrics dashboard with overview, per-step metrics, timeline, and recent failures

Migrations are discovered automatically — when a migrator starts it announces itself to the server, which the console then picks up from `GET /migrations`. You don't need to register anything manually in normal use.

## Tech

Next.js 16, React 19, Tailwind CSS v4. Talks to the server HTTP API via a Next.js rewrite proxy (`/api/*` → `localhost:8080`). Types are generated from the OpenAPI schema (`src/lib/api.gen.ts`).

## Development

```bash
bun install
bun run dev     # starts on http://localhost:3000
```

## API proxy

The console proxies all `/api/*` requests to the server via `next.config.ts` rewrites. No `NEXT_PUBLIC_API_URL` env var is needed — the proxy target is hardcoded to `http://localhost:8080` for local dev.
