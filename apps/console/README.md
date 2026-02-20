# console

The Loom web UI. A Next.js dashboard for browsing registered migrations, starting runs, and tracking progress.

## What it does

- **Dashboard** (`/`) — lists all registered migrations with their status at a glance
- **Migration detail** (`/migrations/:id`) — shows the step sequence, registered targets, run history, and lets you start a new run for a specific target or delete the migration
- **New migration** (`/migrations/new`) — form to manually register a migration definition
- **Run detail** (`/runs/:id`) — shows live step progress for a specific run, including PR links as they open and step pass/fail state

Migrations are discovered automatically — when a worker starts it announces itself to the server, which the console then picks up from `GET /migrations`. You don't need to register anything manually in normal use.

## Tech

Next.js 15, React 19, Tailwind CSS v4. Talks exclusively to the server HTTP API. Types are generated from the OpenAPI schema (`src/lib/api.gen.ts`).

## Development

```bash
bun install
bun dev        # starts on http://localhost:3000
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Server base URL |
