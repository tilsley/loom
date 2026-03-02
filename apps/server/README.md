# server

The Loom orchestration server. It is the central control plane that connects the console, migrators, and Temporal.

## Responsibilities

**1. Migration registry** — receives `POST /registry/announce` from migrators on startup, persists migration definitions and candidate lists in PostgreSQL, serves them to the console.

**2. Run lifecycle** — starts, cancels, and retries Runs on behalf of the console; queries run state and translates step progress back to the API.

**3. Event relay** — receives step callbacks from migrators (`POST /event/:id`) and forwards them as signals into the waiting Run.

**4. Console API** — serves everything the UI needs: migration listing, candidate management, step progress, dry-run previews, metrics dashboard.

The server has **no knowledge of what individual steps do**. It knows that a migration has steps and that they are executed by a named migrator at a given URL — nothing more.

## API

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/migrations` | List registered migrations |
| `GET` | `/migrations/:id` | Get a migration |
| `POST` | `/migrations/:id/candidates` | Submit discovered candidates |
| `GET` | `/migrations/:id/candidates` | List candidates |
| `POST` | `/migrations/:id/candidates/:candidateId/start` | Start a run for a candidate |
| `POST` | `/migrations/:id/candidates/:candidateId/cancel` | Cancel a running candidate |
| `POST` | `/migrations/:id/candidates/:candidateId/retry-step` | Retry a failed step |
| `PATCH` | `/migrations/:id/candidates/:candidateId/inputs` | Update operator-supplied inputs |
| `GET` | `/migrations/:id/candidates/:candidateId/steps` | Get step progress |
| `POST` | `/migrations/:id/dry-run` | Dry-run preview |
| `POST` | `/event/:id` | Migrator callback: step update or completion |
| `POST` | `/registry/announce` | Migrator self-registration on startup |
| `GET` | `/metrics/overview` | Aggregate migration metrics |
| `GET` | `/metrics/steps` | Per-step metrics |
| `GET` | `/metrics/timeline` | Event timeline |
| `GET` | `/metrics/failures` | Recent step failures |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOSTPORT` | `localhost:7233` | Temporal server address |
| `POSTGRES_URL` | _(required)_ | PostgreSQL connection string for migration state and event store |
| `PORT` | `8080` | HTTP listen port |
| `OTEL_ENABLED` | `false` | Enable OpenTelemetry tracing and metrics |
| `OTEL_SERVICE_NAME` | `loom-server` | Service name reported to the OTEL collector |
