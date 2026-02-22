# Loom — Domain Model

This document is the canonical reference for domain terminology used across the server,
worker, and console. When naming things in code, API contracts, and UI copy, use the terms
defined here.

---

## Core Entities

### Migration

A **migration** is a registered definition describing *how* to move a set of targets from one
state to another. It is authored by a worker and registered with the server — either via
pub/sub announcement on startup, or directly via the API.

A migration has:
- A unique **id** (deterministic slug, e.g. `app-chart-migration`)
- A human **name** and optional **description**
- An ordered list of **steps** defining the work to be done
- A list of **run IDs** recording its execution history
- A map of **candidate runs** tracking per-candidate run state

A migration is not tied to a specific set of targets — it is a reusable definition. Targets
are discovered separately (see Candidate).

### Step

A **step** is a single unit of work within a migration. Steps are executed sequentially per
target by a **worker app**.

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
- An **id** (the primary key — logical name of the thing being migrated, e.g. `billing-api`)
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

A **run** is a single execution of a migration against one candidate (target). A run begins
in a `queued` state with no active workflow; once executed, it is tracked as a Temporal
workflow instance.

A run has:
- A **run ID** (human-readable, sortable: `{migrationId}-{unixTimestamp}`)
- A **candidate** (the candidate being migrated, captured at queue time)
- A **status** (see Run Status below)
- A **migration ID** (`migrationId` — back-reference to the parent migration)

---

## Statuses

### Candidate Status

Tracks where a candidate sits in the migration lifecycle.

| Status      | Meaning                                                      |
|-------------|--------------------------------------------------------------|
| `not_started`   | Discovered, no run has been created yet                      |
| `queued`    | A run has been created and is waiting to be executed         |
| `running`   | The run's workflow is actively executing                     |
| `completed` | The run finished successfully                                |
| `failed`    | The run finished with an error                               |

### Run Status

Mirrors candidate status for the run itself (as stored in Temporal / the state store).

| Status      | Meaning                                          |
|-------------|--------------------------------------------------|
| `queued`    | Run record created, Temporal workflow not started |
| `running`   | Temporal workflow is executing                   |
| `completed` | Temporal workflow finished successfully          |
| `failed`    | Temporal workflow finished with an error         |

---

## Actions

| Action        | Actor   | Description                                                        |
|---------------|---------|--------------------------------------------------------------------|
| **Announce**  | Worker  | Register or update a migration definition via pub/sub            |
| **Discover**  | Worker  | Scan a source of truth and submit a candidate list to the server |
| **Queue**     | Console | Create a run instance for a candidate without starting execution |
| **Dequeue**   | Console | Remove a queued run, returning the candidate to not_started          |
| **Execute**   | Console | Start the Temporal workflow for a queued run                     |
| **Complete**  | Worker  | Signal a step as done (success or failure) via the event endpoint|

---

## Lifecycle

```
                    Worker                         Console
                      │                               │
              Announce migration                      │
                      │                               │
              Discover candidates ──────────────► [not_started]
                                                      │
                                               Queue run ──► [queued] ──► Dequeue ──► [not_started]
                                                                │
                                                          Execute run
                                                                │
                                                           [running]
                                                                │
                                                      Steps execute...
                                                                │
                                                           ┌────┴────┐
                                                      [completed] [failed]
```

---

## Key Relationships

```
Migration 1 ──── * Step
Migration 1 ──── * Candidate (via discovery)
Migration 1 ──── * Run (via execution history)
Candidate  1 ──── 0..1 Run (at a point in time)
Run        1 ──── 1 Candidate (id + kind + metadata at time of queuing)
Run        1 ──── * StepResult (accumulated as steps complete)
```

---

## Naming Conventions

| Concept            | Code (Go)              | API (JSON)         | UI (Console)       |
|--------------------|------------------------|--------------------|--------------------|
| Migration id       | `migration.Id`         | `id`               | ID                 |
| Candidate          | `api.Candidate`        | `candidate`        | Candidate          |
| Candidate id       | `candidate.Id`         | `id`               | (displayed as-is)  |
| Candidate kind     | `candidate.Kind`       | `kind`             | —                  |
| GitHub owner/repo  | `metadata["repoName"]` | `metadata.repoName`| —                  |
| Run                | `runId` / `CandidateRun` | `runId`          | Run                |
| Run migration ID   | `record.MigrationID`   | `migrationId`      | —                  |
| Queue a run        | `service.Queue()`      | `POST .../queue`   | "Queue"            |
| Dequeue a run      | `service.Dequeue()`    | `DELETE .../dequeue` | "Remove from queue" |
| Execute a run      | `service.Execute()`    | `POST .../execute` | "Execute"          |
| Step               | `api.StepDefinition`   | `step`             | Step               |
| Worker app         | `step.WorkerApp`       | `workerApp`        | Worker             |
