# Loom — Domain Model

This document is the canonical reference for domain terminology used across the server,
worker, and console. When naming things in code, API contracts, and UI copy, use the terms
defined here.

---

## Core Entities

### Migration

A **migration** is a registered definition describing *how* to move a set of candidates from one
state to another. It is authored by a worker and registered with the server via pub/sub
announcement on startup.

A migration has:
- A unique **id** (deterministic slug, e.g. `app-chart-migration`)
- A human **name** and **description** (both required)
- An ordered list of **steps** defining the work to be done
- An optional list of **required inputs** — each has a `name` (metadata key merged into the candidate at start time, e.g. `"repoName"`) and a `label` (human-readable display label shown in the UI, e.g. `"Repository"`)
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

### Candidate

A **candidate** is a target discovered by a worker as needing migration. Discovery is
performed by a `Discoverer` implementation that scans a source of truth (e.g. a GitOps repo).

A candidate has:
- An **id** (the primary key — stable slug, e.g. `billing-api`)
- A required **kind** (what type of thing it is, e.g. `application`, `kafka-topic` — free-form string set by the discoverer)
- Optional **metadata** (stable descriptive values set by the discoverer, e.g. `repoName`, `team`, `gitopsPath`)
- Optional **files** (grouped file references populated by the discoverer — a list of `FileGroup` objects, each with a `name` context like `"prod"`, `"staging"`, or `"app-repo"`, the repo it belongs to, and a list of file paths + GitHub URLs)
- An optional **status** (`not_started | running | completed`) — stored directly on the candidate

The `id` is the primary key used throughout the server and console. It is a logical identifier,
not a GitHub path. The GitHub repo name lives in `metadata["repoName"]` (set by the worker's
discoverer) and is only needed by the worker when creating PRs. The server and console treat
metadata as an opaque key/value map.

Candidates are submitted by workers and stored embedded within the migration object. They are
the single source of truth for *what* needs migrating and its current status.

`SubmitCandidates` (called by workers on pod restart) uses merge-not-replace semantics: incoming
candidates overwrite only `not_started` ones; `running` or `completed` candidates are preserved
even if absent from the new discovery list.

---

## Statuses

### Candidate Status

Tracks where a candidate sits in the migration lifecycle. There is no `failed` status at the
candidate level — if a step fails, the candidate is reset to `not_started` and can be
previewed and executed again.

| Status        | Meaning                                                    |
|---------------|------------------------------------------------------------|
| `not_started` | Discovered, not yet executed                               |
| `running`     | The workflow is actively executing                         |
| `completed`   | The workflow finished successfully                         |

### Cancelled Attempts

When a running candidate is cancelled, a `CancelledAttempt` record is appended to
the migration. The candidate resets to `not_started`. Cancelled attempts are not surfaced on
the candidate row — they are accessible via the migration detail (admin view).

A `CancelledAttempt` has: `candidateId`, `cancelledAt`.

---

## Actions

| Action        | Actor   | Description                                                         |
|---------------|---------|---------------------------------------------------------------------|
| **Announce**  | Worker  | Register or update a migration definition via pub/sub             |
| **Discover**  | Worker  | Scan a source of truth and submit a candidate list to the server  |
| **Preview**   | Console | Navigate to the preview page; auto-calls dry-run (stateless)      |
| **Start**     | Console | Start the Temporal workflow for a candidate; sets status to `running` |
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
                                                 Preview (dry-run, stateless)
                                                      │
                                                  Start
                                                      │
                                                  [running] ──► Cancel ──► [not_started]
                                                      │                   (+ CancelledAttempt)
                                                  Steps execute...
                                                      │
                                                  [completed]
```

---

## Key Relationships

```
Migration  1 ──── * Step
Migration  1 ──── * Candidate (via discovery, stored on migration object)
Migration  1 ──── * CancelledAttempt (audit log)
Candidate         carries: status (not_started | running | completed)
```

> **Implementation note:** The Temporal workflow instance ID is derived as
> `{migrationId}__{candidateId}`. This is an internal detail — it is not a domain concept and
> does not appear in the API or UI.

---

## Naming Conventions

| Concept            | Code (Go)                  | API (JSON)           | UI (Console)         |
|--------------------|----------------------------|----------------------|----------------------|
| Migration id       | `migration.Id`             | `id`                 | ID                   |
| Candidate          | `api.Candidate`            | `candidate`          | Candidate            |
| Candidate id       | `candidate.Id`             | `id`                 | (displayed as-is)    |
| Candidate kind     | `candidate.Kind`           | `kind`               | —                    |
| Candidate status   | `candidate.Status`         | `status`             | Status dot / badge   |
| GitHub repo name   | `metadata["repoName"]`     | `metadata.repoName`  | —                    |
| Preview a candidate| —                          | —                    | "Preview"            |
| Start a candidate  | `service.Start()`          | `POST .../start`     | "Start"              |
| Cancel a candidate | `service.Cancel()`         | `POST .../cancel`    | "Cancel"             |
| Step               | `api.StepDefinition`       | `step`               | Step                 |
| Worker app         | `step.WorkerApp`           | `workerApp`          | Worker               |
