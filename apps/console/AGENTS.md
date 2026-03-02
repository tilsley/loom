# apps/console — Agent Instructions

Next.js 16 (Turbopack) dashboard for Loom. All commands use **bun**.

## Running

```bash
bun install           # install deps
bun run dev           # dev server on :3000 (proxies /api/* → localhost:8080)
bun run build         # production build
```

The server must be running on `:8080` for API calls to work. The proxy is configured via `rewrites` in `next.config.ts` — no env var needed in dev.

## Testing

```bash
bun run test          # vitest run (single pass)
bun run test:watch    # watch mode
```

Tests live alongside source in `src/**/*.test.{ts,tsx}`. Environment is `jsdom`. Mock `fetch` with `vi.stubGlobal('fetch', ...)` — see `src/lib/__tests__/api.test.ts` for the pattern.

## Linting & formatting

```bash
bun run typecheck     # tsc --noEmit — always run after TS changes
bun run lint          # oxlint + eslint (run both; oxlint is faster, eslint adds rules)
bun run lint:fix      # auto-fix
bun run format        # Biome formatter (write)
bun run format:check  # Biome formatter (check only)
bun run knip          # detect unused exports/imports
```

The linter pipeline is: **oxlint first** (handles ~100 rules fast), then **eslint** for rules oxlint doesn't cover. Don't disable one without the other.

## Code structure

```
src/
├── app/                        Next.js App Router pages
│   ├── layout.tsx              Root layout
│   ├── page.tsx                Dashboard /
│   ├── metrics/page.tsx        Metrics dashboard
│   └── migrations/
│       ├── [id]/page.tsx       Migration detail (polling, candidate table)
│       ├── [id]/preview/[candidateId]/page.tsx   Dry-run preview + start
│       └── [id]/candidates/[candidateId]/steps/page.tsx   Live step progress
├── components/
│   ├── ui/                     Low-level UI primitives (Button, Dialog, Table, Badge…)
│   ├── candidate-row.tsx       Single candidate row with status + actions
│   ├── candidate-table.tsx     Filterable, searchable candidate list
│   ├── migration-card.tsx      Migration summary card for dashboard
│   ├── step-timeline.tsx       Step progress timeline
│   ├── preview-panel.tsx       Dry-run preview panel
│   ├── progress-bar.tsx        Candidate completion progress bar
│   ├── metrics-chart.tsx       Recharts wrappers for metrics page
│   ├── status-filter.tsx       Status toggle filter
│   ├── command-palette.tsx     Cmd+K command palette
│   ├── sidebar.tsx             App sidebar navigation
│   └── file-diff-view.tsx      File diff display for previews
├── lib/
│   ├── api.gen.ts              GENERATED — do not edit (openapi-typescript)
│   ├── api.ts                  Hand-written API wrappers around api.gen.ts
│   ├── routes.ts               ROUTES constant — all Next.js hrefs defined here
│   ├── hooks.ts                Shared hooks (useMigrationPolling, etc.)
│   ├── formatting.ts           Display formatting helpers
│   ├── filtering.ts            Candidate filtering logic
│   ├── inputs.ts               Required input prefill logic
│   ├── steps.ts                Step status helpers
│   ├── stats.ts                Candidate stats computation
│   ├── url.ts                  URL/search-param helpers
│   └── utils.ts                General utilities (cn, etc.)
└── contexts/
    ├── theme-context.tsx
    └── migrations-context.tsx
```

## Key conventions

**All pages are client components** (`"use client"`). Route params are accessed via `useParams()` hook — never server component `params` props.

**Modals use Radix `<Dialog>`** from `@/components/ui`. Follow the pattern in `migrations/[id]/page.tsx` (overview modal, cancel confirmation, preview inputs). The command palette is the one exception — it uses `createPortal` directly.

**Candidate actions:** `onPreview` / `onCancel` flow from the page → `CandidateTable` → `CandidateRow`. Add new action props at all three levels.

**Button inside a Link** — always call both `e.preventDefault()` and `e.stopPropagation()` to prevent navigation.

**Polling** — migration detail page polls dynamically: 2s when any candidate is `running`, 5s otherwise, via `setInterval` in `useEffect`. After a mutation (e.g. cancel), call `fetchCandidates()` immediately so the UI updates without waiting for the next poll.

**API client (`src/lib/api.ts`)** — all fetch calls go here. Follow the existing pattern: throw `new Error(await res.text())` on non-ok responses; throw `ConflictError` on 409. Import types from `api.gen.ts` via `api.ts` re-exports.

**Generated types** — `api.gen.ts` is regenerated from `schemas/openapi.yaml` by running `make generate-ts` (or `bun run generate`). Never edit it. After schema changes, regenerate before running the app.

## UI components

`src/components/ui/` contains shadcn-style primitives, re-exported from `ui/index.ts`:

- `Button` — variants: `default` (subtle primary), `primary` (solid primary), `danger` (destructive), `outline` (muted), `success` (completed); sizes: `sm`, `default`, `lg`, `icon`
- `buttonVariants` — CVA helper for applying button styles to non-button elements (e.g. `<Link>`)
- `Dialog`, `DialogContent`, `DialogHeader`, `DialogFooter`, `DialogTitle`, `DialogDescription` — Radix-based modal
- `Table`, `TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell`
- `Accordion`, `Badge`, `Input`, `Skeleton`, `Sheet`, `Tooltip`, `ToggleGroup`, `ErrorBoundary`, `Toaster`

## TypeScript config

- `target: ES2022`, `lib: ["dom", "dom.iterable", "esnext"]`
- `strict: true`, `noEmit: true`
- Path alias: `@/` → `src/`

## Notable tooling

| Tool | Config | Purpose |
|---|---|---|
| oxlint | `.oxlintrc.json` | Fast linting (~100 rules) |
| ESLint | `eslint.config.mjs` | Flat config, React/TS rules |
| Biome | `biome.json` | Formatting only (linter disabled), 100-char lines |
| Knip | `knip.json` | Unused exports/imports detection |
| Vitest | `vitest.config.ts` | Unit tests, jsdom, globals |
