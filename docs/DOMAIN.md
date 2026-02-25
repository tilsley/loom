# Domain Model

This document defines the core concepts of Loom and how they relate. Use it as
the authoritative reference when naming things in code, writing comments, or
designing new features.

---

## Concepts

### Migration

A **Migration** is the registered definition of a repeatable process — the
*what* and *how* of a class of work. It is authored and announced by a
**Migrator** and stored in the server. It contains:

- A set of **Steps** (ordered, with types and config)
- A list of discovered **Candidates** (the subjects to be migrated)
- Required operator inputs (e.g. a target image tag)
- The migrator URL the server should dispatch steps to (`migratorUrl`)

A Migration is a plan, not an execution. Running a Migration against a
Candidate produces a **Run**.

> **Code note:** The generated API type `api.Migration` serves as the
> MigrationSpec. The `MigrationAnnouncement` type is the delivery event a
> migrator sends on startup to register or update the spec.

---

### Candidate

A **Candidate** is a subject that a Migration can be applied to — typically an
application, service, repository, or topic. Candidates are discovered and
registered by the Migrator alongside the Migration definition.

A Candidate has:
- A stable **id** (the logical name, e.g. `checkout`)
- A **kind** (e.g. `application`, `service`)
- **Metadata** (arbitrary key-value pairs used by step handlers)
- **Files** (file groups discovered from the candidate's repository)
- A **status**: `not_started | running | completed`

The status tracks whether a Run has been attempted, not whether the Migration
itself succeeded — step-level outcomes are recorded separately in each Run.

---

### Migrator

A **Migrator** is an external service that knows how to execute a specific
class of Migration. It is responsible for:

- Announcing itself (and its Migration definition) to the server on startup
- Discovering Candidates and submitting them
- Executing Steps when dispatched by the server

The server is Migrator-agnostic — it dispatches steps over HTTP and receives
completion callbacks. The `migratorUrl` (registered at announce time) is the
only coupling point.

> **Example:** `app-chart-migrator` is a Migrator that handles Helm chart
> upgrades across ArgoCD applications.

> **Code note:** The `MigratorNotifier` port is the server-side abstraction for
> dispatching a step to a Migrator. "Worker" in this codebase always refers to
> the Temporal worker process inside the server — never to a Migrator.

---

### Run

A **Run** is a single execution of a Migration against one Candidate. It
sequences all of the Migration's Steps for that Candidate and tracks their
outcomes.

A Run is identified by `RunID = "{migrationId}__{candidateId}"`. Because each
Candidate is run at most once per Migration, this ID is stable and
deterministic.

Internally, a Run is implemented as a Temporal workflow execution. This is an
implementation detail: the domain model does not expose Temporal vocabulary.
The `ExecutionEngine` port abstracts the durable execution runtime.

The `MigrationManifest` is the snapshot of the Migration definition plus the
specific Candidate, passed as input when a Run starts. It is the blueprint for
that execution.

---

### Steps

A **Step** is one unit of work within a Migration. Steps are defined in the
Migration and executed per Candidate, in order.

Two representations:

| Type | Purpose |
|---|---|
| `StepDefinition` | The plan — name, type, migrator app, config |
| `StepResult` | The outcome — status, metadata, PR URL |

A Step has a **type** that determines which handler the Migrator routes to
(e.g. `disable-base-resource-prune`, `manual-review`). The server treats all
step types uniformly — type routing is the Migrator's responsibility.

Step statuses progress through:

```
in_progress → open → merged
           → awaiting_review → completed
           → failed → (retry) → ...
```

---

## How they fit together

```
Migrator (external service)
  └── announces ──► Migration (spec)
                      ├── Steps (definitions)
                      └── Candidates (subjects)
                            │
                            └── Run (one per candidate)
                                  ├── input: MigrationManifest (snapshot of spec + candidate)
                                  └── output: StepResults (one per step)
                                        │
                                        └── dispatched to ──► Migrator (executes each step)
```

---

## Vocabulary rules

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
