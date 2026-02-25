---
name: architecture-guide
description: Explains the server's layered architecture and where to add new code. Use when the user asks where something belongs, which file to edit, which layer owns something, or how the layers relate to each other.
---

## Server layers (`apps/server`)

Four layers — each may only import inward. Never skip a layer.

```
HTTP (api/)          ← Gin handlers. Translates HTTP ↔ service calls. No business logic.
Service (service.go) ← Use-case orchestrator. Calls ports only. No framework imports.
Execution (execution/) ← Temporal workflow + activities. Steps are sequenced here.
Infrastructure (adapters/) ← Concrete implementations of port interfaces.
```

### Where to add things

| Task | File(s) |
|------|---------|
| New HTTP endpoint | `internal/migrations/api/routes.go` (register) + appropriate handler file |
| Candidate endpoints | `internal/migrations/api/candidates.go` |
| Migration endpoints | `internal/migrations/api/migrations.go` |
| Event/webhook endpoints | `internal/migrations/api/events.go` |
| New use-case logic | `internal/migrations/service.go` |
| New port interface | `internal/migrations/ports.go` |
| Domain types / errors | `internal/migrations/domain.go` |
| Redis state operations | `internal/migrations/adapters/redis_store.go` |
| Worker dispatch (HTTP) | `internal/migrations/adapters/http_notifier.go` |
| Dry-run dispatch (HTTP) | `internal/migrations/adapters/http_dryrun.go` |
| Temporal workflow logic | `internal/migrations/execution/workflow.go` |
| Temporal activities | `internal/migrations/execution/activity.go` |
| Shared API types | `pkg/api/types.gen.go` — **generated, edit `schemas/openapi.yaml` instead** |

### Port interfaces (in `ports.go`)

```go
ExecutionEngine  — StartRun, GetStatus, RaiseEvent, CancelRun  →  Temporal
MigrationStore   — Save, Get, List, SetCandidateStatus, SaveCandidates, GetCandidates  →  Redis
WorkerNotifier   — Dispatch  →  HTTP POST /dispatch-step on the worker
DryRunner        — DryRun  →  HTTP POST /dry-run on the worker
```

The service layer depends only on these interfaces — never on Redis, Temporal, or Gin directly.

### Key rules

- **HTTP handlers**: call `svc.*`, map domain errors to HTTP status codes, nothing else
- **Service**: call port interfaces only; no knowledge of Gin/Temporal/Redis
- **Execution layer**: Temporal is an intentional framework dependency here — it's not swappable
- **Adapters**: implement port interfaces; import external SDKs freely
- **Workflow instance ID**: always `{migrationId}__{candidateId}` — this is an internal detail, not exposed in the API

### Console (`apps/console`)

```
app/migrations/                          ← list page
app/migrations/[id]/                     ← migration detail (candidates)
app/migrations/[id]/candidates/[candidateId]/steps/  ← step timeline
app/migrations/[id]/preview/             ← dry-run diff view
components/step-timeline.tsx             ← visual timeline component
lib/api.gen.ts                           ← generated types (don't edit directly)
lib/client.ts                            ← HTTP client helper
```
