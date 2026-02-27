# server

The Loom orchestration server. It is the central control plane that connects the console, migrators, and Temporal.

## Responsibilities

**1. Migration registry** — receives `POST /registry/announce` from migrators on startup, persists migration definitions and candidate lists in Redis, serves them to the console.

**2. Workflow lifecycle** — starts, cancels, and signals Temporal workflows on behalf of the console; queries workflow state and translates step progress back to the API.

**3. Event relay** — receives step callbacks from migrators (`POST /event/:id`) and forwards them as Temporal signals into the waiting workflow.

**4. Console API** — serves everything the UI needs: migration listing, candidate management, step progress, dry-run previews.

The server has **no knowledge of what individual steps do**. It knows that a migration has steps and that they are executed by a named migrator at a given URL — nothing more.

## API

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/migrations` | List registered migrations |
| `GET` | `/migrations/:id` | Get a migration |
| `POST` | `/migrations/:id/candidates` | Submit discovered candidates |
| `GET` | `/migrations/:id/candidates` | List candidates |
| `POST` | `/migrations/:id/candidates/:cid/start` | Start a run for a candidate |
| `POST` | `/migrations/:id/candidates/:cid/cancel` | Cancel a running candidate |
| `POST` | `/migrations/:id/candidates/:cid/retry-step` | Retry a failed step |
| `GET` | `/migrations/:id/candidates/:cid/steps` | Get step progress |
| `POST` | `/migrations/:id/dry-run` | Dry-run preview |
| `POST` | `/event/:id` | Migrator callback: step update or completion |
| `POST` | `/registry/announce` | Migrator self-registration on startup |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOSTPORT` | `localhost:7233` | Temporal server address |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | _(empty)_ | Redis password |
| `PORT` | `8080` | HTTP listen port |
| `OTEL_ENABLED` | `false` | Enable OpenTelemetry tracing and metrics |
