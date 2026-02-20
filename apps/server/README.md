# server

The Loom orchestration server. It is the central coordinator for all migration runs.

## What it does

- Runs a **Temporal workflow** (`MigrationOrchestrator`) that sequences migration steps across target repos, one step at a time, waiting for each to complete before moving to the next
- Exposes a **REST API** for the console to register migrations, start runs, and query progress
- Receives **worker announcements** via Dapr pub/sub (`migration-registry` topic) and saves migration definitions to its state store — workers self-register on startup
- Dispatches step work to the migration worker via Dapr pub/sub (`migration-steps` topic)
- Receives **callbacks** from the worker when a PR is opened (`POST /event/:id/pr-opened`) and when a step completes (`POST /event/:id`), which signal the waiting workflow to advance
- Implements a **saga**: if any step fails, completed steps are compensated in reverse order

## Key concepts

The server has **no knowledge of what individual steps do**. It only knows the step names and sequence declared by the worker. All migration domain logic lives in the worker.

## API

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/migrations` | List registered migrations |
| `POST` | `/migrations` | Register a migration manually |
| `GET` | `/migrations/:id` | Get a specific migration |
| `DELETE` | `/migrations/:id` | Delete a migration |
| `POST` | `/migrations/:id/run` | Start a run for one target repo |
| `POST` | `/event/:id` | Worker callback: step completed |
| `POST` | `/event/:id/pr-opened` | Worker callback: PR is open |
| `POST` | `/registry/announce` | Dapr delivers worker announcements here |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOSTPORT` | `localhost:7233` | Temporal server address |
| `PORT` | `8080` | HTTP listen port |
