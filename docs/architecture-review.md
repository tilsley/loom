# Architecture Review

This document captures a review of the current Loom design — what's working well, what's over-engineered, and the deeper understanding that emerged from working through the design questions. It feeds directly into ADR-0002.

---

## What's right

### The core separation of concerns

The fundamental decision — a generic management server that knows nothing about what migrations do, and workers that register themselves — is the right call and well executed. Adding a new migration type requires only a new worker. The server, console, and operational tooling work without modification. See ADR-0001 for the full reasoning.

### Temporal for workflow orchestration

Temporal earns its keep. Once you read the workflow, the retry/signal/cancel mechanics justify the dependency:

- The retry loop blocks waiting for either a `step-completed` signal or a `retry` signal or workflow cancellation — this is genuinely stateful behaviour
- `ctx.Done()` lets the workflow detect cancellation while blocked mid-step, triggering a clean candidate reset
- The real-time query handler (`progress`) lets the console poll live step results without polling a database
- Durable execution means the workflow survives server restarts mid-migration

Replicating this correctly with a database + polling approach would produce messier, less reliable code. Temporal is not over-engineering here.

### Hexagonal architecture in the server

The service layer has zero framework imports. All Temporal, Dapr, and Gin code sits at the edges behind port interfaces. `service.go` reads cleanly as business logic. The test stubs (`memStore`, `stubEngine`) are straightforward because the interfaces are narrow.

### OpenAPI as the single source of truth

One schema generates both Go types (`pkg/api/types.gen.go`) and TypeScript types (`apps/console/src/lib/api.gen.ts`). Server, workers, and console all share the same contract. This is the right approach.

---

## What's over-engineered

### 1. Dapr

Dapr is the clearest case of unnecessary complexity.

**What it provides here:**
- Pub/sub abstraction over Redis (step dispatch to workers)
- State store abstraction over Redis (migration/candidate state)
- Service invocation (dry-run calls to workers)

**What it costs:**
- A sidecar container alongside every app
- A placement service (`daprio/dapr`)
- `dapr run` wrappers in every make target
- Component YAML files for each transport
- Five or more services running locally just to develop
- Four hops (server → sidecar → Redis → sidecar → worker) for what should be one

**Why the abstraction benefit doesn't materialise:**
The abstraction Dapr promises — swap Redis for another broker without changing application code — already exists in the Go port interfaces (`WorkerNotifier`, `DryRunner`, `MigrationStore`). Dapr duplicates an abstraction that's already there. And you're already committed to Redis; there is no realistic scenario where the broker is swapped.

**What replaces it:** See ADR-0002.

---

### 2. Dual source of truth for candidate status

Candidate status is stored in two places:

- **Redis** (via Dapr state store): `not_started | running | completed`
- **Temporal**: workflow execution state

When they disagree — which happens after a Temporal restart — `GetCandidates` has to reconcile them by probing Temporal for every candidate currently marked `running` in Redis:

```go
// service.go
if _, err := s.engine.GetStatus(ctx, workflowID); err != nil {
    var notFound WorkflowNotFoundError
    if errors.As(err, &notFound) {
        // Stale workflow — reset to not_started
        _ = s.store.SetCandidateStatus(ctx, migrationID, c.Id, api.CandidateStatusNotStarted)
    }
}
```

This is defensive patching of a design gap. The workflow already writes `completed` back to Redis via the `UpdateCandidateStatus` activity. Applying the same principle to `not_started` (on cancel or failure) would mean Redis is always eventually consistent with Temporal, and the reconciliation loop in `GetCandidates` becomes unnecessary.

---

### 3. `MigrationManifest.Candidates` is always length 1

The workflow loops over `manifest.Candidates`, but `service.Start` always passes a single candidate:

```go
manifest := api.MigrationManifest{
    MigrationId: migrationID,
    Candidates:  []api.Candidate{candidate},  // always one
    Steps:       m.Steps,
}
```

The slice implies the workflow can handle batches. It can't cleanly — each candidate requires its own workflow ID, its own signal channel names, and its own retry state. The inner loop is never more than one iteration. The field should be a single `Candidate`, making the manifest's intent explicit and removing a false implication about batch execution.

---

## Deeper understanding from design questions

### How would server–worker communication work without Dapr?

Currently Dapr handles three server → worker interactions:

| Interaction | Current | Without Dapr |
|---|---|---|
| Step dispatch | Pub/sub via Redis | Direct HTTP to worker |
| Dry-run | Dapr service invocation | Direct HTTP to worker |
| State store | Dapr Redis adapter | go-redis directly |

Note that worker → server communication is **already direct HTTP** — the announcement (`POST /registry/announce`) and step completion callback (`POST /event/{id}`) both bypass Dapr entirely. Removing Dapr makes server → worker consistent with worker → server.

**The missing piece is worker address registration.** In the current design the server dispatches via pub/sub and never needs to know where workers are running. Moving to direct HTTP requires the server to know each worker's base URL. The natural place to supply this is the announcement:

```json
{
  "id": "app-chart-migration",
  "workerApp": "app-chart-migrator",
  "workerUrl": "http://app-chart-migrator:3001",
  ...
}
```

The server stores a `workerApp → URL` map and makes plain `net/http` calls for dispatch and dry-run.

**Does this add a network requirement that didn't exist before?** No. Dry-run already requires the server to reach the worker via Dapr service invocation. Workers are already required to be network-reachable from the server. Moving step dispatch to HTTP doesn't add a new requirement.

---

### Are rollbacks managed by Dapr or Temporal?

**Dapr plays no role in rollbacks.** It is a transport layer — it moves messages and stores state. It has no concept of compensation or saga.

Rollbacks are Temporal's territory. The workflow already has the skeleton:

```go
// workflow.go
defer func() {
    if !failed {
        return
    }
    resetCandidate()
    // No saga compensation — PRs cannot be automatically undone.
}()
```

On failure, the candidate is reset to `not_started`. This is the only "rollback" currently implemented.

**Full saga compensation** — closing the PRs that were opened before the failure, reverting commits — would be implemented as compensation activities added to the Temporal workflow, running in reverse step order. Temporal guarantees those activities execute even if the server restarts mid-compensation. Removing Dapr has zero effect on this capability.

---

### Is the migration definition a structure Temporal understands?

No. Temporal has no understanding of the migration definition. It is purely custom.

The migration definition is a Go struct (`api.MigrationAnnouncement`) hardcoded in the worker's announcement handler. When a migration is started, the server packages the steps and candidate into a `MigrationManifest` and passes it to Temporal as opaque JSON workflow input.

```
Temporal knows:     "run MigrationOrchestrator with this JSON payload"
Temporal does not:  know what a step is, what workerApp means, how dispatch works
```

`MigrationOrchestrator` is a **generic interpreter** — it loops over whatever steps arrive in the manifest and dispatches each one to the named worker. The intelligence is in the Go workflow code, not in anything Temporal-specific.

This has an interesting implication: because the migration definition is just data that the generic workflow interprets, it could eventually be loaded from a YAML file at worker startup rather than hardcoded in Go — without changing anything about the Temporal workflow or the server. The generic interpreter pattern already supports it.

---

## Summary of proposed changes

| Issue | Change | Impact |
|---|---|---|
| Dapr over-engineering | Replace with direct Redis (state) + HTTP (dispatch + dry-run) | Removes sidecars, placement service, `dapr run`, component YAMLs; simplifies local dev from 5+ services to 3 |
| Worker URL registration | Add `workerUrl` to `MigrationAnnouncement` | Small schema addition; enables direct HTTP dispatch |
| Dual state reconciliation | Temporal activities own all status writes; remove reconciliation loop in `GetCandidates` | Eliminates state inconsistency on restart |
| `MigrationManifest.Candidates []` | Change to single `Candidate` field | Removes false batch implication; breaks no real behaviour |
