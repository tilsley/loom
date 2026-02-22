# Loom — Domain Model

This document is the canonical reference for domain terminology used across the server,
worker, and console. When naming things in code, API contracts, and UI copy, use the terms
defined here.

---

## Core Entities

### Migration

A **migration** is a registered definition describing *how* to move a set of candidates from one
state to another. It is authored by a worker and registered with the server — either via
pub/sub announcement on startup, or directly via the API.

A migration has:
- A unique **id** (deterministic slug, e.g. `app-chart-migration`)
- A human **name** and optional **description**
- An ordered list of **steps** defining the work to be done
- A map of **candidate runs** tracking per-candidate run state
- An optional list of **cancelled attempts** (audit log)

A migration is not tied to a specific set of candidates — it is a reusable definition. Candidates
are discovered separately (see Candidate).

### Step

A **step** is a single unit of work within a migration. Steps are executed sequentially per
candidate by a **worker app**.

A step has:
- A **name** (unique within the migration)
- An optional **description** (human-readable explanation shown in the console)
- A **worker app** identifier (which worker handles it)
- Optional **config** (key/value pairs passed to the worker)
- Optional **files** (URLs of files that will be touched)

### Candidate

A **candidate** is a target discovered by a worker as needing migration. Discovery is
performed by a `Discoverer` implementation that scans a source of truth (e.g. a GitOps repo).

A candidate has:
- An **id** (the primary key — stable slug, e.g. `billing-api`)
- An optional **kind** (what type of thing it is, e.g. `application`, `kafka-topic` — free-form string set by the discoverer)
- Optional **metadata** (stable descriptive values set by the discoverer, e.g. `repoName`, `team`, `gitopsPath`)
- Optional **state** (observed values at discovery time, e.g. `currentChart`, `currentVersion`)

The `id` is the primary key used throughout the server and console. It is a logical identifier,
not a GitHub path. The GitHub `owner/repo` string lives in `metadata["repoName"]` (set by the
worker's discoverer) and is only needed by the worker when creating PRs. The server and console
treat metadata as an opaque key/value map.

Candidates are submitted by workers and stored per migration. They are the source of truth
for *what* needs migrating.

When returned from the API, candidates are enriched into `CandidateWithStatus` — the status
is derived server-side by joining the candidate list against the migration's candidate run map,
and is not stored on the candidate itself.

### Run

A **run** represents the execution of a migration against one candidate. Each candidate has at
most one run per migration — a migration is intended to be run once.

A run has:
- A **run ID** (deterministic: `{migrationId}__{candidateId}`, e.g. `app-chart-migration__billing-api`)
- A **status** (see Candidate Status below)

The run ID is fully recoverable from the migration ID and candidate ID — no separate lookup
record is needed. Temporal uses it as the workflow instance ID, which naturally prevents
duplicate runs for the same candidate.

---

## Statuses

### Candidate Status

Tracks where a candidate sits in the migration lifecycle. There is no `failed` status at the
candidate level — if a step fails, the candidate is reset to `not_started` and can be
re-queued.

| Status        | Meaning                                                      |
|---------------|--------------------------------------------------------------|
| `not_started` | Discovered, not yet queued                                   |
| `queued`      | Queued for execution (dry run visible, workflow not started) |
| `running`     | The workflow is actively executing                           |
| `completed`   | The workflow finished successfully                           |

### Cancelled Attempts

When a running or queued candidate is cancelled, a `CancelledAttempt` record is appended to
the migration. The candidate resets to `not_started`. Cancelled attempts are not surfaced on
the candidate row — they are accessible via the migration detail (admin view).

A `CancelledAttempt` has: `runId`, `candidateId`, `cancelledAt`.

---

## Actions

| Action        | Actor   | Description                                                         |
|---------------|---------|---------------------------------------------------------------------|
| **Announce**  | Worker  | Register or update a migration definition via pub/sub             |
| **Discover**  | Worker  | Scan a source of truth and submit a candidate list to the server  |
| **Queue**     | Console | Reserve a run for a candidate; enables dry-run preview            |
| **Dequeue**   | Console | Remove a queued run, returning the candidate to `not_started`     |
| **Execute**   | Console | Start the Temporal workflow for a queued run                      |
| **Cancel**    | Console | Stop a running workflow; resets candidate to `not_started` and records a `CancelledAttempt` |
| **Complete**  | Worker  | Signal a step as done (success or failure) via the event endpoint |

---

## Lifecycle

```
                    Worker                         Console
                      │                               │
              Announce migration                      │
                      │                               │
              Discover candidates ──────────────► [not_started]
                                                      │
                                         Queue ──► [queued] ──► Dequeue ──► [not_started]
                                                       │
                                                  Execute
                                                       │
                                                  [running] ──► Cancel ──► [not_started]
                                                       │                  (+ CancelledAttempt)
                                                  Steps execute...
                                                       │
                                                  [completed]
```

---

## Key Relationships

```
Migration  1 ──── * Step
Migration  1 ──── * Candidate (via discovery)
Migration  1 ──── * CancelledAttempt (audit log)
Candidate  1 ──── 0..1 CandidateRun (current state only)
CandidateRun     holds: status
Run ID           computed: {migrationId}__{candidateId}
```

---

## Naming Conventions

| Concept            | Code (Go)              | API (JSON)           | UI (Console)         |
|--------------------|------------------------|----------------------|----------------------|
| Migration id       | `migration.Id`         | `id`                 | ID                   |
| Candidate          | `api.Candidate`        | `candidate`          | Candidate            |
| Candidate id       | `candidate.Id`         | `id`                 | (displayed as-is)    |
| Candidate kind     | `candidate.Kind`       | `kind`               | —                    |
| GitHub owner/repo  | `metadata["repoName"]` | `metadata.repoName`  | —                    |
| Run ID             | `RunID(mId, cId)`      | `runId`              | Run                  |
| Candidate run      | `api.CandidateRun`     | `candidateRuns[id]`  | —                    |
| Queue a run        | `service.Queue()`      | `POST .../queue`     | "Queue"              |
| Dequeue a run      | `service.Dequeue()`    | `DELETE .../dequeue` | "Remove from queue"  |
| Execute a run      | `service.Execute()`    | `POST .../execute`   | "Execute"            |
| Cancel a run       | `service.Cancel()`     | `POST .../cancel`    | "Cancel"             |
| Step               | `api.StepDefinition`   | `step`               | Step                 |
| Worker app         | `step.WorkerApp`       | `workerApp`          | Worker               |
