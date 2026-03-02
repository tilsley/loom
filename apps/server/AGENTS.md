# apps/server — Agent Instructions

Go HTTP + Temporal workflow orchestration server. Entry point: `main.go`.

## Running

```bash
# Via Makefile (recommended):
make run
# Equivalent:
cd apps/server && go run .

# Prerequisites:
make temporal       # Temporal dev server (port 7233, UI :8088)
docker compose up -d  # Redis, PostgreSQL, Temporal (containerised)
```

## Testing

```bash
go test ./...
go test ./apps/server/internal/migrations/...           # service + store + migrator
go test ./apps/server/internal/migrations/execution/... # workflow + activities
```

Tests use stub implementations of all port interfaces — no external services required. The stubs live alongside the tests (`service_test.go`, `testhelpers_test.go`, `workflow_test.go`).

## Layered architecture

```
handler/          inbound HTTP (Gin)
  ↓ calls
service.go        orchestration (use-case logic + guards)
  ↓ calls via port interfaces
execution/        Temporal workflow + activities
store/            PostgreSQL (migrations, candidates, events)
migrator/         outbound HTTP (step dispatch + dry-run)
```

**Rule:** the service layer (`service.go`) only imports domain types and port interfaces. It never imports Temporal, PostgreSQL, or Gin packages directly.

## Key files

| File | Purpose |
|---|---|
| `main.go` | Wires all layers: logger → OTEL → Temporal → Postgres → service → Gin |
| `internal/migrations/errors.go` | Sentinel error types (`MigrationNotFoundError`, `CandidateNotFoundError`, etc.) |
| `internal/migrations/run.go` | Run identity helpers (`RunID`, `ParseRunID`), signal names, `RunStatus` type |
| `internal/migrations/ports.go` | Port interfaces: `ExecutionEngine`, `MigrationStore`, `MigratorNotifier`, `DryRunner`, `EventStore` |
| `internal/migrations/service.go` | All use-case logic — edit here for business logic changes |
| `internal/migrations/handler/routes.go` | Gin route registration |
| `internal/migrations/handler/candidates.go` | Candidate lifecycle handlers (start, cancel, retry, inputs, steps) |
| `internal/migrations/handler/migrations.go` | Migration CRUD + candidate submission handlers |
| `internal/migrations/handler/events.go` | Announce + migrator callback handlers |
| `internal/migrations/handler/metrics.go` | Metrics dashboard handlers |
| `internal/migrations/store/pg_migration_store.go` | PostgreSQL migration + candidate store (implements `MigrationStore`) |
| `internal/migrations/store/pg_event_store.go` | PostgreSQL event store (implements `EventStore`) |
| `internal/migrations/migrator/http_notifier.go` | Outbound HTTP (implements `MigratorNotifier`) |
| `internal/migrations/migrator/http_dryrun.go` | Outbound HTTP (implements `DryRunner`) |
| `internal/migrations/execution/workflow.go` | Temporal `MigrationOrchestrator` workflow |
| `internal/migrations/execution/activity.go` | `DispatchStep`, `UpdateCandidateStatus`, `RecordEvent` activities |
| `internal/platform/telemetry/telemetry.go` | OTEL setup — opt-in via `OTEL_ENABLED=true` |
| `internal/platform/validation/` | OpenAPI request validation middleware |

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `TEMPORAL_HOSTPORT` | `localhost:7233` | |
| `POSTGRES_URL` | _(required)_ | PostgreSQL connection string for migration state and event store |
| `OTEL_ENABLED` | `false` | Set `true` to emit traces/metrics |
| `OTEL_SERVICE_NAME` | `loom-server` | |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | Required if OTEL enabled |

## Adding a new endpoint

1. Add path + schema to `schemas/openapi.yaml`
2. Run `make generate` (updates `pkg/api/types.gen.go` and console's `api.gen.ts`)
3. Add service method to `service.go` (depend only on port interfaces)
4. Add Gin handler to the appropriate `handler/*.go` file and register in `handler/routes.go`
5. Add tests in `service_test.go` (stub ports) and `handler/*_test.go`

## Go linting

`.golangci.yml` at repo root — 30+ linters, 120-char line limit, strict error handling. Run `make lint-go` before committing. Key rules that catch people out:

- `errcheck` — all errors must be handled
- `errorlint` — use `errors.Is`/`errors.As`, not `==`
- `contextcheck` — pass `ctx` through the full call chain
- `gocognit` — cognitive complexity ≤ 15 per function
- `gosec` — no G404 exclusion needed but others apply
