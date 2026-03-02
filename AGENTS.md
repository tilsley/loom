# Loom — Agent Instructions

## Repo overview

Monorepo for **Loom**, a Temporal-backed migration orchestration platform.

| App | Path | Language |
|---|---|---|
| Orchestration server | `apps/server` | Go |
| Web console | `apps/console` | TypeScript / Next.js |
| Helm chart migrator | `apps/migrators/app-chart-migrator` | Go |
| Mock GitHub API | `apps/mock-github` | Go |
| Shared Go types | `pkg/` | Go |
| OpenAPI schema | `schemas/` | YAML |

Single `go.mod` at repo root — all Go apps share one module (`github.com/tilsley/loom`).

## Domain model

Read `DOMAIN.md` for canonical terminology before touching anything. Key terms:

- **Migration** — a registered definition with an ordered list of Steps
- **Step** — a single unit of work executed by a named migrator app
- **Candidate** — a subject discovered by a migrator that needs migrating (e.g. an application slug)
- **Candidate status** — `not_started` | `running` | `completed` (no `failed`; step failure keeps candidate `running`)
- **Run** — one execution of a migration against a candidate
- **Run ID** — internal identifier derived as `{migrationId}__{candidateId}`; not surfaced in the API

## Running the stack

```bash
make setup       # one-time: installs deps, generates types
make dev         # starts everything (Temporal, server, migrator, mock-github, console)
make dev-otel    # same + OTEL_ENABLED=true + Grafana LGTM on :3002
```

Infrastructure only (no app code):
```bash
docker compose up -d       # Temporal, PostgreSQL, Redis, Temporal UI
make temporal              # Temporal dev server (SQLite, UI on :8088)
make reset                 # flush Redis + delete Temporal DB + truncate event store
```

## Code generation

The OpenAPI spec at `schemas/openapi.yaml` is the source of truth. After any change to it:

```bash
make generate          # regenerates both Go and TypeScript types
make generate-go       # → pkg/api/types.gen.go   (oapi-codegen)
make generate-ts       # → apps/console/src/lib/api.gen.ts  (openapi-typescript)
```

Never edit `*.gen.go` or `api.gen.ts` by hand.

## Testing

```bash
make test              # go test ./...  (all Go)
go test ./apps/server/...
go test ./apps/migrators/...

cd apps/console && bun run test          # vitest run
cd apps/console && bun run test:watch    # watch mode
```

## Linting & formatting

```bash
make lint              # golangci-lint (Go) + oxlint+eslint (console)
make lint-fix          # auto-fix where possible
make lint-go           # Go only
make console-lint      # TS only
make console-format    # Biome formatter (TS)
make console-typecheck # tsc --noEmit
```

Go linting is strict (30+ linters, see `.golangci.yml`). Generated files (`*.gen.go`) are excluded. Test files have a more lenient profile.

## Architecture rule

The server follows a strict layered import rule — see `apps/server/ARCHITECTURE.md`. Never import infrastructure packages from the service layer; always go through the port interfaces defined in `internal/migrations/ports.go`.

## Key conventions

- **Port interfaces over concrete types** — all Temporal, HTTP, and external interactions are hidden behind interfaces in the service layer.
- **Table-driven tests in Go** — use `memStore`, `stubEngine`, etc. stub implementations (see `service_test.go`).
- **No auto-commit** — stage changes and describe them; let the human confirm before committing.
- **After OpenAPI changes** — always run `make generate` before running or testing anything.
- **Run ID is internal** — never expose the `migrationId__candidateId` pattern to users or in API responses.
