# Server Architecture

```
┌─────────────────────────────────────────────┐
│  handler/           inbound HTTP             │  UI + migrator callbacks
├─────────────────────────────────────────────┤
│  service.go         orchestration            │  use-case logic + guards
├─────────────────────────────────────────────┤
│  execution/         Temporal workflow        │  step sequencing + signals
├─────────────────────────────────────────────┤
│  store/             PostgreSQL                │  migration + candidate + event state
│  migrator/          outbound HTTP             │  step dispatch + dry-run
└─────────────────────────────────────────────┘
```

## Packages

### `handler/`
Gin route handlers. Translates HTTP requests into service calls and maps domain errors to HTTP status codes. No business logic.

Inbound callers:
- Console (UI) — migration management, candidate control, dry-run
- Migrators — announce registration, step completion callbacks

### `service.go` + `ports.go`
The use-case orchestrator. Enforces business rules (e.g. guard against starting an already-running candidate), coordinates between the execution engine and the store. No framework imports — depends only on the port interfaces defined in `ports.go`.

Port interfaces:
- `ExecutionEngine` — start, query, and cancel runs; raise signals
- `MigrationStore` — persist and retrieve migration + candidate state
- `MigratorNotifier` — dispatch step requests to migrators
- `DryRunner` — invoke a migrator synchronously for a dry-run preview
- `EventStore` — record lifecycle events and query metrics (step events, timelines, failures)

### `execution/`
The Temporal workflow and its activities. Sequences steps across candidates, waits for step-completion signals, handles retries, and runs compensation on cancellation. Framework-coupled by design — Temporal is a core dependency here, not a swappable adapter.

Activities use the same `MigratorNotifier` and `MigrationStore` port interfaces as the service layer.

### `store/`
- `PGMigrationStore` — implements `MigrationStore` using PostgreSQL. Migrations and candidates stored in separate tables; candidates are independently queryable.
- `PGEventStore` — implements `EventStore` using PostgreSQL. Records step lifecycle events and serves metrics queries.

### `migrator/`
Outbound HTTP clients that implement the `MigratorNotifier` and `DryRunner` ports. POSTs directly to the migrator's base URL (registered at announce time via `migratorUrl`).

## Supporting files

- `errors.go` — sentinel error types returned by the service layer (`MigrationNotFoundError`, `CandidateNotFoundError`, `CandidateAlreadyRunError`, `CandidateNotRunningError`, `RunNotFoundError`, `InvalidInputKeyError`)
- `run.go` — run identity helpers (`RunID`, `ParseRunID`), signal name helpers, `RunStatus` type and `RuntimeStatus` constants

## Shared types (`pkg/api/`)
Generated from `schemas/openapi.yaml` via oapi-codegen. All layers share these types — they are the wire contract between the server, migrators, and the console.

## Import rules

| Package | May import |
|---|---|
| `handler/` | `service.go` (via interface), `pkg/api`, domain errors |
| `service.go` | `pkg/api`, port interfaces (`ports.go`), `errors.go`, `run.go` |
| `execution/` | port interfaces, `pkg/api`, `run.go` |
| `store/` | `pkg/api`, pgx |
| `migrator/` | `pkg/api` |
| `platform/temporal/` | port interfaces (`RunStatus`, `RunNotFoundError`), Temporal SDK |
| `platform/postgres/` | `pkg/api` |
| `platform/telemetry/` | OTEL SDK |
| `platform/logger/` | slog |
| `platform/validation/` | `pkg/api`, OpenAPI spec |

No package imports above itself. The service layer has no knowledge of Gin, PostgreSQL, or Temporal.

## Platform packages (`internal/platform/`)

Infrastructure concerns shared across the server:

- `temporal/` — implements `ExecutionEngine` port; Temporal client + worker setup
- `postgres/` — PostgreSQL connection pool; implements `EventStore` port
- `telemetry/` — OTEL tracer/meter provider; opt-in via `OTEL_ENABLED=true`
- `logger/` — structured logging (slog)
- `validation/` — OpenAPI request validation middleware for Gin
