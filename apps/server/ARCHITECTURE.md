# Server Architecture

```
┌─────────────────────────────────────────────┐
│  handler/           inbound HTTP             │  UI + migrator callbacks
├─────────────────────────────────────────────┤
│  service.go         orchestration            │  use-case logic + guards
├─────────────────────────────────────────────┤
│  execution/         Temporal workflow        │  step sequencing + signals
├─────────────────────────────────────────────┤
│  store/             Redis                    │  migration + candidate state
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

### `execution/`
The Temporal workflow and its activities. Sequences steps across candidates, waits for step-completion signals, handles retries, and runs compensation on cancellation. Framework-coupled by design — Temporal is a core dependency here, not a swappable adapter.

Activities use the same `MigratorNotifier` and `MigrationStore` port interfaces as the service layer.

### `store/`
`RedisMigrationStore` — implements `MigrationStore` using go-redis. Keyed by migration ID; candidate state stored as part of the migration document.

### `migrator/`
Outbound HTTP clients that implement the `MigratorNotifier` and `DryRunner` ports. POSTs directly to the migrator's base URL (registered at announce time via `migratorUrl`).

## Supporting files

- `errors.go` — sentinel error types returned by the service layer (`MigrationNotFoundError`, `CandidateNotFoundError`, etc.)
- `run.go` — run identity helpers (`RunID`, `ParseRunID`), signal name helpers, `RunStatus` type and `RuntimeStatus` constants

## Shared types (`pkg/api/`)
Generated from `schemas/openapi.yaml` via oapi-codegen. All layers share these types — they are the wire contract between the server, migrators, and the console.

## Import rules

| Package | May import |
|---|---|
| `handler/` | `service.go` (via interface), `pkg/api`, domain errors |
| `service.go` | `pkg/api`, port interfaces (`ports.go`), `errors.go`, `run.go` |
| `execution/` | port interfaces, `pkg/api`, `run.go` |
| `store/` | `pkg/api`, go-redis |
| `migrator/` | `pkg/api` |
| `platform/temporal/` | port interfaces (`RunStatus`, `RunNotFoundError`), Temporal SDK |

No package imports above itself. The service layer has no knowledge of Gin, Redis, or Temporal.
