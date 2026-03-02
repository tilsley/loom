# Loom — Domain Model

This document is the canonical reference for domain terminology used across the server,
migrator, and console. When naming things in code, API contracts, and UI copy, use the terms
defined here.

---

## Core Entities

### Migration

A **Migration** is a registered definition describing *how* to move a set of candidates from one
state to another. It is authored by a **Migrator** and registered with the server via HTTP POST
to `/registry/announce` on startup.

A Migration has:
- A unique **id** (deterministic slug, e.g. `app-chart-migration`)
- A human **name** and **description** (both required)
- An ordered list of **Steps** defining the work to be done
- An optional list of **required inputs** — each has a `name` (metadata key merged into the candidate at start time, e.g. `"repoName"`) and a `label` (human-readable display label shown in the UI, e.g. `"Repository"`)
- An optional **overview** (list of strings describing high-level phases, shown on the migration detail page)
- The **migratorUrl** the server dispatches steps to

A Migration is a plan, not an execution. Running a Migration against a Candidate produces a
**Run**.

> **Code note:** The generated API type `api.Migration` serves as the spec. The
> `MigrationAnnouncement` type is the delivery event a Migrator sends on startup to register
> or update the spec.

### Step

A **Step** is a single unit of work within a Migration. Steps are executed sequentially per
Candidate by a **migrator app**.

Two representations:

| Type | Purpose |
|---|---|
| `StepDefinition` | The plan — name, type, migrator app, config |
| `StepState` | The outcome — status, metadata |

A Step has:
- A **name** (unique within the Migration)
- An optional **description** (human-readable explanation shown in the console)
- A **migrator app** identifier (`migratorApp` — which migrator handles it)
- Optional **config** (key/value pairs passed to the migrator)

A Step has a **type** that determines which handler the Migrator routes to
(e.g. `disable-base-resource-prune`, `manual-review`). The server treats all step types
uniformly — type routing is the Migrator's responsibility.

Step statuses progress through:

```
in_progress → pending → succeeded
                      → merged
           → failed → (retry) → ...
```

`pending` is the only intermediate status — the Migrator sends it to update visible state
(e.g. a PR URL in metadata) while the Run keeps waiting. `merged` and `succeeded` are both
terminal success states.

### Candidate

A **Candidate** is a subject that a Migration can be applied to — typically an application,
service, repository, or topic. Candidates are discovered and registered by the Migrator
alongside the Migration definition.

A Candidate has:
- An **id** (the primary key — stable slug, e.g. `billing-api`)
- A required **kind** (what type of thing it is, e.g. `application`, `kafka-topic` — free-form string set by the discoverer)
- Optional **metadata** (stable descriptive values set by the discoverer, e.g. `repoName`, `team`, `gitopsPath`)
- Optional **files** (grouped file references populated by the discoverer — a list of `FileGroup` objects, each with a `name` context like `"prod"`, `"staging"`, or `"app-repo"`, the repo it belongs to, and a list of file paths + GitHub URLs)
- Optional per-candidate **steps** (`Steps *[]StepDefinition`) — overrides the Migration-level steps when present
- A **status**: `not_started | running | completed`

The `id` is the primary key used throughout the server and console. It is a logical identifier,
not a GitHub path. The GitHub repo name lives in `metadata["repoName"]` (set by the Migrator's
discoverer) and is only needed by the Migrator when creating PRs. The server and console treat
metadata as an opaque key/value map.

Candidates are submitted by Migrators and stored per-migration. They are the single source of
truth for *what* needs migrating and its current status.

`SubmitCandidates` (called by Migrators on pod restart) uses merge-not-replace semantics:
incoming candidates overwrite only `not_started` ones; `running` or `completed` candidates are
preserved even if absent from the new discovery list.

### Migrator

A **Migrator** is an external service that knows how to execute a specific class of Migration.
It is responsible for:

- Announcing itself (and its Migration definition) to the server on startup
- Discovering Candidates and submitting them
- Executing Steps when dispatched by the server

The server is Migrator-agnostic — it dispatches steps over HTTP and receives completion
callbacks. The `migratorUrl` (registered at announce time) is the only coupling point.

> **Example:** `app-chart-migrator` is a Migrator that handles Helm chart upgrades across
> ArgoCD applications.

> **Code note:** The `MigratorNotifier` port is the server-side abstraction for dispatching a
> step to a Migrator. "Worker" in this codebase always refers to the Temporal worker process
> inside the server — never to a Migrator.

### Run

A **Run** is a single execution of a Migration against one Candidate. It sequences all of the
Migration's Steps (or the Candidate's per-candidate Steps) for that Candidate and tracks their
outcomes.

A Run is identified by `RunID = "{migrationId}__{candidateId}"`. Because each Candidate is run
at most once per Migration, this ID is stable and deterministic.

Internally, a Run is implemented as a Temporal workflow execution. This is an implementation
detail: the domain model does not expose Temporal vocabulary. The `ExecutionEngine` port
abstracts the durable execution runtime.

The `MigrationManifest` is the snapshot of the Migration definition plus the specific Candidate,
passed as input when a Run starts. It is the blueprint for that execution.

---

## Statuses

### Candidate Status

Tracks where a Candidate sits in the migration lifecycle. There is no `failed` status at the
Candidate level. When a step fails, the Run stays alive and the Candidate remains `running`.
The failed step is visible in the steps view (rendered in red). The operator can either **retry**
(re-dispatches the step to the Migrator) or **cancel** (resets the Candidate to `not_started`).

| Status        | Meaning                                    |
|---------------|--------------------------------------------|
| `not_started` | Discovered, not yet executed               |
| `running`     | A Run is actively executing                |
| `completed`   | The Run finished successfully              |

### Step Status

Tracks the outcome of a single Step within a Run.

| Status        | Meaning                                                  |
|---------------|----------------------------------------------------------|
| `in_progress` | Step has been dispatched; waiting for a callback         |
| `pending`     | Intermediate update from Migrator (e.g. PR URL in metadata) |
| `succeeded`   | Step completed successfully                              |
| `merged`      | Step completed via merge (terminal success)              |
| `failed`      | Step failed; operator can retry or cancel                |

---

## Actions

| Action        | Actor    | Description                                                         |
|---------------|----------|---------------------------------------------------------------------|
| **Announce**  | Migrator | Register or update a Migration definition via HTTP POST             |
| **Discover**  | Migrator | Scan a source of truth and submit a candidate list to the server    |
| **Preview**   | Console  | Navigate to the preview page; auto-calls dry-run (stateless)        |
| **Start**     | Console  | Start a Run for a Candidate; sets status to `running`               |
| **Cancel**    | Console  | Stop a running Run; resets Candidate to `not_started`               |
| **Retry**     | Console  | Re-dispatch a failed step to the Migrator; Candidate stays `running`|
| **Complete**  | Migrator | Signal a step as done (success or failure) via the event endpoint   |

---

## Lifecycle

```
                    Migrator                       Console
                      │                               │
              Announce Migration                      │
                      │                               │
              Discover Candidates ──────────────► [not_started]
                                                      │
                                                 Preview (dry-run, stateless)
                                                      │
                                                  Start
                                                      │
                                                  [running] ──► Cancel ──► [not_started]
                                                  Steps execute...
                                                      │
                                                  [completed]
```

---

## How They Fit Together

```
Migrator (external service)
  └── announces ──► Migration (spec)
                      ├── Steps (definitions)
                      ├── Overview (optional phase descriptions)
                      └── Candidates (subjects)
                            │
                            └── Run (one per Candidate)
                                  ├── input: MigrationManifest (snapshot of spec + candidate)
                                  └── output: StepStates (one per step)
                                        │
                                        └── dispatched to ──► Migrator (executes each step)
```

---

## Key Relationships

```
Migration  1 ──── * Step            (ordered definitions)
Migration  1 ──── * Candidate       (via discovery)
Candidate  1 ──── * Step?           (optional per-candidate override)
Candidate         carries: status   (not_started | running | completed)
Run        =      Migration × Candidate   (one execution)
```

> **Implementation note:** The Run ID is derived as `{migrationId}__{candidateId}`. This is
> an internal detail — it is not a domain concept and does not appear in the API or UI.

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
| Step definition    | `api.StepDefinition`       | `step`               | Step                 |
| Migrator app       | `step.MigratorApp`         | `migratorApp`        | Migrator             |

---

## Vocabulary Rules

Use these terms consistently across code, comments, and the UI:

| Concept | Use | Avoid |
|---|---|---|
| The registered plan | **Migration** | "workflow definition", "job" |
| The subject being migrated | **Candidate** | "target", "repo", "app" (use `kind` for specifics) |
| One execution of a migration | **Run** | "workflow", "execution", "job run" |
| One unit of work | **Step** | "task", "action", "stage" |
| The snapshot passed to a Run | **MigrationManifest** | "run input", "payload" |
| The external execution service | **Migrator** | "worker" (reserved for Temporal internals) |
| The durable execution engine | **ExecutionEngine** (port) | "Temporal", "workflow engine" |
