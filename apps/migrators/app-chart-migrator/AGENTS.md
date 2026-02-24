# apps/migrators/app-chart-migrator — Agent Instructions

A Loom worker app that orchestrates Helm chart migrations across GitOps repositories. It discovers candidates, executes migration steps (swap chart, disable sync, cleanup), creates PRs, and signals completion back to the server.

## Running

```bash
# Via Makefile (recommended — starts with Dapr sidecar):
make migrator
# Equivalent:
dapr run --app-id app-chart-migrator --app-port 3001 --dapr-http-port 3501 -- go run .

# Prerequisites: Redis + Dapr placement + server must be running
```

The app listens on `:3001` and registers itself with the Loom server on startup.

## Testing

```bash
go test ./...
go test ./apps/migrators/app-chart-migrator/...
```

GitHub interactions use an in-memory mock (`internal/adapters/github/inmem.go`). No real GitHub or Git operations in tests.

## How it fits into Loom

```
Startup → announce migration definition to server
        → discover candidates (scan GitOps repo for old Helm charts)
        → POST /migrations/{id}/candidates to server

Server dispatches step → Dapr pub/sub → handler/dispatch.go
Worker executes step → creates PR → POST /event/{id} back to server
```

The server controls sequencing; the worker only executes one step at a time per candidate.

## Key files

| File | Purpose |
|---|---|
| `main.go` | Wires GitHub client, Dapr, discovery, dry-run, steps, Gin router |
| `internal/handler/webhook.go` | `/migrations/announce` — registers migration with server on startup |
| `internal/handler/dispatch.go` | Dapr pub/sub handler — receives step dispatch, runs step, signals completion |
| `internal/handler/dryrun.go` | `/migrations/{id}/dry-run` — simulates without creating PRs |
| `internal/discovery/appchartdiscoverer.go` | Scans GitOps repo, returns list of candidates |
| `internal/steps/handler.go` | Dispatches to the correct step implementation |
| `internal/steps/swap_chart.go` | Replaces old chart reference in YAML |
| `internal/dryrun/runner.go` | Simulates steps, captures file diffs |
| `internal/gitrepo/` | Clone, checkout, read, and record changes to GitHub repos |
| `internal/adapters/github/adapter.go` | Creates branches, commits, opens PRs |
| `internal/platform/pending/store.go` | Tracks in-flight step callbacks |

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `3001` | HTTP port |
| `DAPR_HTTP_PORT` | `3501` | Set by Dapr sidecar automatically |
| `LOOM_URL` | `http://localhost:8080` | Server base URL |
| `GITHUB_API_URL` | `http://localhost:9090` | Point to `mock-github` in dev |
| `GITOPS_REPO` | — | `owner/repo` format (e.g. `tilsley/gitops`) |
| `ENVS` | — | Comma-separated environments (e.g. `dev,staging,prod`) |
| `GITHUB_TOKEN` | — | Token auth (local dev / CI) |
| `GITHUB_APP_ID` | — | App auth (production) — use instead of token |
| `GITHUB_APP_INSTALLATION_ID` | — | App auth |
| `GITHUB_APP_PRIVATE_KEY_PATH` | — | App auth |

## HTTP endpoints

| Method | Path | Handler |
|---|---|---|
| `POST` | `/migrations/announce` | Register migration definition with server |
| `POST` | `/migrations/{id}/{step}` | Execute a step (Dapr pub/sub dispatch) |
| `POST` | `/migrations/{id}/dry-run` | Simulate all steps, return file diffs |
| `POST` | `/event/{id}` | Receive completion callback from server (if needed) |

## Conventions

**Candidate ID** is the repo slug (e.g. `billing-api`). Candidate `kind` is `"repository"`. Metadata includes `repoName`, `team`, and environment keys.

**Dry-run is stateless** — it clones the repo, simulates changes in memory using `gitrepo/recording.go`, and returns diffs without writing to GitHub.

**Step execution** creates a branch, commits changes, opens a PR, then signals the server via `POST /event/{workflowId}`. The workflow ID is `{migrationId}__{candidateId}`.

**GitHub auth** — use token auth locally (set `GITHUB_TOKEN`). The `mock-github` app (`make mock-github`) simulates GitHub API responses without network calls.

**Adding a new step:**
1. Add the step name to the migration definition in `handler/webhook.go`
2. Implement the step in `internal/steps/` following the existing pattern
3. Register it in `internal/steps/handler.go`
4. Add dry-run simulation in `internal/dryrun/runner.go`
