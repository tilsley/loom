---
name: domain-model
description: Domain terminology for the Loom migration platform. Use when discussing migrations, candidates, steps, statuses, or any Loom-specific concept to ensure consistent naming across code, API, and UI.
---

## Core terms

**Migration** — a registered definition describing *how* to move a set of targets from one state to another. Authored by a worker, registered via `/registry/announce`. Has an ordered list of steps and optional required inputs. Not tied to specific candidates — it's reusable.

**Candidate** — a single target discovered as needing migration (e.g. one service, one repo). Has:
- `id` — stable slug, primary key (e.g. `billing-api`)
- `kind` — what type of thing it is (e.g. `application`, `kafka-topic`)
- `metadata` — opaque key/value map set by discoverer (e.g. `repoName`, `team`)
- `files` — grouped file references (e.g. per-env ArgoCD manifests)
- `status` — `not_started | running | completed`

**Step** — a single unit of work within a migration, executed sequentially per candidate by a worker app. Defined on the Migration; not stored per-candidate.

**Worker** — an external service that implements the migration logic. Announces itself on startup, receives step dispatches via HTTP, signals completion via the event endpoint.

**Run** — a Temporal workflow execution for one candidate. Internal detail: instance ID is `{migrationId}__{candidateId}`.

## Candidate statuses

| Status | Meaning |
|--------|---------|
| `not_started` | Discovered, not yet executed |
| `running` | Workflow is actively executing |
| `completed` | Workflow finished successfully |

There is no `failed` status at the candidate level. When a step fails, the candidate stays `running` — the operator can retry the step or cancel the candidate (which resets it to `not_started`).

## Actions

| Action | Actor | What happens |
|--------|-------|--------------|
| **Announce** | Worker | Register/update a migration definition |
| **Discover** | Worker | Submit a candidate list to the server |
| **Preview** | Operator | Trigger a stateless dry-run; see file diffs before execution |
| **Start** | Operator | Launch Temporal workflow; candidate → `running` |
| **Cancel** | Operator | Stop workflow; candidate → `not_started` |
| **Retry** | Operator | Re-dispatch a failed step; candidate stays `running` |
| **Complete** | Worker | Signal step done (success or failure) via event endpoint |

## Naming conventions

| Concept | Go | API JSON | UI |
|---------|----|-----------|----|
| Migration id | `migration.Id` | `id` | ID |
| Candidate id | `candidate.Id` | `id` | shown as-is |
| GitHub repo | `metadata["repoName"]` | `metadata.repoName` | — |
| Start | `service.Start()` | `POST .../start` | "Start" |
| Cancel | `service.Cancel()` | `POST .../cancel` | "Cancel" |
| Step name | `stepDef.Name` | `name` | Step |

## Key relationships

```
Migration  1 ──── * StepDefinition
Migration  1 ──── * Candidate
Candidate       carries: status
```

Candidates are stored embedded within the migration object in Redis. `SubmitCandidates` uses merge-not-replace semantics: `running` and `completed` candidates are never overwritten by re-discovery.
