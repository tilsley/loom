# ADR-0001: Decouple migration management from migration execution

**Status:** Accepted

---

## Context

Large-scale migrations — renaming Kafka topics, upgrading Helm charts, rotating secrets across hundreds of services — share the same operational concerns regardless of what they actually do:

- Track which targets have been migrated and which haven't
- Allow an operator to preview, start, cancel, and monitor progress
- Execute work sequentially per target, retrying failed steps without losing progress
- Surface status to engineers without requiring them to run scripts locally

The naive approach embeds all of this — tracking, UI, execution logic — into a single bespoke tool per migration type. That works once but doesn't scale: each new migration type reinvents the management layer, the UI, and the orchestration mechanics.

The question was how to build a platform that handles any migration type without needing to be changed each time a new migration is introduced.

---

## Decision

Split the system into two independently deployable parts connected by a lightweight contract:

**1. A generic management server** (`apps/server`) that owns:
- The migration registry (what migrations exist, what their steps are)
- Candidate state (which targets exist, what status each is in)
- Workflow orchestration (sequencing steps, handling retries, cancel/resume)
- The REST API consumed by the console

The server has no knowledge of what any migration actually does. It knows that a migration has steps and that steps are executed by named worker apps — nothing more.

**2. Worker apps** (e.g. `apps/migrators/app-chart-migrator`) that own:
- Discovery: scanning a source of truth (GitOps repo, Kafka cluster, etc.) and submitting the candidate list to the server
- Execution: performing each step (cloning repos, creating PRs, rotating credentials, etc.)
- Dry-run: simulating the migration and returning file diffs for preview

**The registration pattern** binds them together:

- Workers announce their migration definition to the server on startup via Dapr pub/sub (`/registry/announce`). The announcement includes the migration ID, name, description, ordered step list, and required inputs.
- Workers submit discovered candidates to the server via `POST /migrations/{id}/candidates`. The server stores candidates but treats metadata as an opaque key/value map — it never interprets it.
- The server dispatches step execution to the correct worker via pub/sub (worker app is named per-step in the migration definition). Workers signal completion back via `POST /event/{id}`.

This means: **adding a new migration type requires only a new worker app**. The server, console, and all operational tooling work without modification.

---

## Consequences

**Positive**

- New migration types are self-contained. A team writes a worker, deploys it, and it appears in the console on next startup — no server or console changes required.
- The management layer (state, sequencing, UI) is tested and hardened once. Workers only need to implement discovery, dry-run, and step execution.
- Workers can be written in any language that can speak HTTP and Dapr pub/sub. The contract is defined by `schemas/openapi.yaml`.
- Migrations are observable and operable through a single console regardless of type — no per-migration dashboards.
- The server's Temporal-backed workflow engine handles retry, cancel, and progress tracking durably. Workers don't need to implement any of this.

**Negative / trade-offs**

- Workers must implement the announcement and discovery contract on startup. This is a small but non-trivial integration cost for each new migration type.
- The server stores candidates embedded within the migration object in Redis. This is simple and fast but means candidates are not independently queryable — they always come through the migration. This is acceptable for the current scale (hundreds to low thousands of candidates per migration).
- Because workers are separate deployments, a worker restart re-runs discovery. The server uses merge-not-replace semantics to protect in-progress and completed candidates, but the operator needs to be aware that discovery is re-entrant.
- Temporal is a hard dependency of the server. It is not abstracted away — the execution layer is framework-coupled by design. Swapping orchestration engines would require replacing the execution layer.

---

## Alternatives considered

**Single deployable with pluggable step handlers**
Embed all migration logic in the server behind a plugin or strategy interface. Rejected because it tightly couples deployment of the management platform with each new migration type, requires the same team to maintain all workers, and makes language choice uniform.

**Generic script runner (e.g. GitHub Actions, Argo Workflows)**
Use an existing workflow platform with custom scripts per step. Rejected because these tools lack the domain-specific concepts (candidates, status tracking per target, dry-run preview) and would require building the management UI and candidate tracking on top anyway.

**Event sourcing the candidate state**
Model each status transition as an event rather than storing current state directly. Rejected as over-engineered for the access patterns here — the console only needs current state, and there is no requirement for a full audit trail of every transition.
