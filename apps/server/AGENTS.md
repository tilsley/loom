# apps/server — Agent Instructions

Go HTTP + Temporal workflow orchestration server. Entry point: `main.go`.

## Running

```bash
# Via Makefile (recommended — starts with Dapr sidecar):
make run
# Equivalent:
dapr run --app-id loom --app-port 8080 --dapr-http-port 3500 -- go run .

# Prerequisites:
make temporal       # Temporal dev server (port 7233, UI :8088)
docker compose up -d redis placement  # Redis + Dapr placement
```

## Testing

```bash
go test ./...
go test ./internal/migrations/...           # service + adapters
go test ./internal/migrations/execution/... # workflow + activities
```

Tests use stub implementations of all port interfaces — no external services required. The stubs live alongside the tests (`service_test.go`, `http_test.go`, `workflow_test.go`).

## Layered architecture

```
HTTP (adapters/http.go)
  ↓ calls
Service (migrations/service.go)
  ↓ calls via port interfaces
Execution (migrations/execution/) ← Temporal workflow + activities
Infrastructure (adapters/dapr*.go, platform/) ← concrete implementations
```

**Rule:** the service layer (`service.go`) only imports domain types and port interfaces. It never imports Temporal, Dapr, or Gin packages directly.

## Key files

| File | Purpose |
|---|---|
| `main.go` | Wires all layers: logger → OTEL → Temporal → Dapr → adapters → service → Gin |
| `internal/migrations/domain.go` | Domain types and sentinel errors |
| `internal/migrations/ports.go` | The 4 port interfaces: `WorkflowEngine`, `MigrationStore`, `WorkerNotifier`, `DryRunner` |
| `internal/migrations/service.go` | All use-case logic — edit here for business logic changes |
| `internal/migrations/adapters/http.go` | Gin handlers — translate HTTP ↔ service calls ↔ domain errors |
| `internal/migrations/adapters/dapr_store.go` | Redis state store (implements `MigrationStore`) |
| `internal/migrations/adapters/dapr_bus.go` | Redis pub/sub (implements `WorkerNotifier`) |
| `internal/migrations/execution/workflow.go` | Temporal `MigrationOrchestrator` workflow |
| `internal/migrations/execution/activity.go` | `DispatchStep`, `UpdateTargetRunStatus` activities |
| `internal/platform/telemetry/telemetry.go` | OTEL setup — opt-in via `OTEL_ENABLED=true` |
| `internal/platform/validation/` | OpenAPI request validation middleware |

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `TEMPORAL_HOSTPORT` | `localhost:7233` | |
| `DAPR_GRPC_PORT` | `50001` | Set by Dapr sidecar automatically |
| `OTEL_ENABLED` | `false` | Set `true` to emit traces/metrics |
| `OTEL_SERVICE_NAME` | `loom-server` | |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | Required if OTEL enabled |
| `OTEL_EXPORTER_OTLP_INSECURE` | — | Set `true` for local LGTM stack |

## Dapr components

`dapr/components/pubsub.yaml` — Redis pub/sub, topic `migration-steps`
`dapr/components/statestore.yaml` — Redis state store with actor mode

## Adding a new endpoint

1. Add path + schema to `schemas/openapi.yaml`
2. Run `make generate` (updates `pkg/api/types.gen.go` and console's `api.gen.ts`)
3. Add service method to `service.go` (depend only on port interfaces)
4. Add Gin handler to `adapters/http.go` and register in `RegisterRoutes`
5. Add tests in `service_test.go` (stub ports) and `http_test.go`

## Go linting

`.golangci.yml` at repo root — 30+ linters, 120-char line limit, strict error handling. Run `make lint-go` before committing. Key rules that catch people out:

- `errcheck` — all errors must be handled
- `errorlint` — use `errors.Is`/`errors.As`, not `==`
- `contextcheck` — pass `ctx` through the full call chain
- `gocognit` — cognitive complexity ≤ 15 per function
- `gosec` — no G404 exclusion needed but others apply
