# Server Architecture

The server is organised into four layers. Each layer may only import inward — never outward.

```
┌─────────────────────────────────────────────┐
│  HTTP layer         adapters/http.go         │  inbound requests
├─────────────────────────────────────────────┤
│  Service layer      migrations/service.go    │  use-case logic
├─────────────────────────────────────────────┤
│  Execution layer    migrations/execution/    │  Temporal workflow + activities
├─────────────────────────────────────────────┤
│  Infrastructure     adapters/dapr*.go        │  Dapr pub/sub, state store, service invocation
└─────────────────────────────────────────────┘
```

## Layers

### HTTP (`internal/migrations/adapters/http.go`)
Gin route handlers. Translates HTTP requests into service calls and maps domain errors to status codes. No business logic.

### Service (`internal/migrations/`)
The application's use-case orchestrator. Contains `service.go` (use cases), `ports.go` (interfaces), and `domain.go` (domain types and errors). Has no framework imports — depends only on the port interfaces defined in the same package.

The port interfaces are:
- `WorkflowEngine` — start, query, cancel, and signal Temporal workflows
- `MigrationStore` — persist and retrieve migration state
- `WorkerNotifier` — dispatch step requests to external workers
- `DryRunner` — invoke a worker synchronously for a dry run

### Execution (`internal/migrations/execution/`)
The Temporal workflow and its activities. This is the runtime execution engine — it sequences steps across candidates, handles signals, and runs saga compensation on failure. It is framework-coupled by design: Temporal is a core dependency, not a swappable adapter.

Activities use the same `WorkerNotifier` and `MigrationStore` port interfaces as the service layer.

### Infrastructure (`internal/migrations/adapters/dapr*.go`, `internal/platform/temporal/`)
Concrete implementations of the port interfaces:
- `DaprBus` → `WorkerNotifier` (pub/sub step dispatch)
- `DaprMigrationStore` → `MigrationStore` (state store)
- `DaprDryRunAdapter` → `DryRunner` (service invocation)
- `TemporalEngine` → `WorkflowEngine`

## Import rules

| Layer | May import |
|---|---|
| HTTP | Service layer, `pkg/api` |
| Service | `pkg/api`, port interfaces (same package) |
| Execution | Service layer (ports + domain), `pkg/api` |
| Infrastructure | `pkg/api`, external SDKs (Dapr, Temporal) |

No layer imports above itself. The service layer has no knowledge of Gin, Dapr, or Temporal.

## Shared types (`pkg/api/`)
Generated from `schemas/openapi.yaml` via oapi-codegen. All layers share these types — they are the data contracts between the server, workers, and the console.
