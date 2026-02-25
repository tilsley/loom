---
name: new-migrator
description: Scaffold a new migrator worker for Loom. Use when the user wants to create a new migration type with its own worker app.
disable-model-invocation: true
argument-hint: [migrator-name]
---

Scaffold a new Loom migrator worker named `$ARGUMENTS`. Use `apps/migrators/app-chart-migrator` as the canonical reference — mirror its structure.

## Directory layout

```
apps/migrators/<migrator-name>/
├── main.go                          # startup: announce + discovery + gin server
├── internal/
│   ├── steps/
│   │   ├── handler.go               # Handler interface + Result type + shared helpers
│   │   ├── registry.go              # map[string]Handler — all step types
│   │   └── <step_name>.go           # one file per step type
│   ├── discovery/
│   │   ├── discoverer.go            # finds candidates from the source of truth
│   │   └── runner.go                # submits candidates to the Loom server
│   ├── handler/
│   │   ├── dispatch.go              # POST /dispatch-step handler
│   │   ├── webhook.go               # POST /webhooks/<source> (if needed)
│   │   └── dryrun.go                # POST /dry-run handler
│   ├── dryrun/
│   │   └── runner.go                # iterates steps via registry, returns FileDiffs
│   └── platform/
│       ├── loom/
│       │   └── client.go            # HTTP client for the Loom server
│       └── pending/
│           └── store.go             # Redis store for manual-review callbacks
└── go.mod                           # inherits from repo root go.mod
```

## Key patterns from app-chart-migrator

### Announcement (main.go)
On startup, POST to `{loomURL}/registry/announce` with a `MigrationAnnouncement`:
```go
api.MigrationAnnouncement{
    Id:          "<migrator-name>",
    Name:        "<Human Name>",
    Description: "...",
    Steps:       []api.StepDefinition{ /* one per step type */ },
    WorkerUrl:   workerURL, // the server calls this back
}
```

Retry the announcement up to 10 times with 2-second backoff — the server may not be ready.

### Step Handler interface
```go
type Handler interface {
    Execute(ctx context.Context, /* source client */, cfg *Config, req api.DispatchStepRequest) (*Result, error)
}
```

`Result` describes the PR/change to create. For manual-review steps, return `&Result{Instructions: "..."}` with empty Owner/Repo.

### Dispatch handler
Receives `api.DispatchStepRequest` from the Loom server. Looks up the handler via `steps.Lookup(req.Type)`. On success, posts `api.StepEvent{Status: "completed"}` back to `{loomURL}/events/{eventName}`. On failure, posts `Status: "failed"`.

### Dry-run runner
Iterates all steps in `api.DryRunRequest`. For each step, calls `steps.Lookup()` — skips unknown types. Accumulates file content in an overlay so each step sees the result of previous steps. Returns `api.DryRunResult` with per-step file diffs.

### Manual-review callback
For steps awaiting approval, save `{callbackId: req.CallbackId, eventName: req.EventName}` to Redis. When the external trigger fires (webhook, etc.), read from Redis and POST the completion event to the Loom server.

## Environment variables to support

| Var | Default | Purpose |
|-----|---------|---------|
| `LOOM_URL` | `http://localhost:8080` | Loom server URL |
| `WORKER_URL` | `http://localhost:300N` | This worker's public URL (for server callbacks) |
| `PORT` | `300N` | Listen port |
| `REDIS_ADDR` | `localhost:6379` | For pending callback store |

## Add to Makefile

Add a target in the root Makefile so `make dev` can start the new worker alongside others. Follow the pattern of the `migrator` target.

## Verify

```bash
make vet
make test
```
