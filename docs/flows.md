# Loom — Key Operation Flows

Sequence diagrams for the main operations in Loom.

**Common participants:**

| Alias | Component | Role |
|---|---|---|
| `Con` | loom-console | Operator UI |
| `H` | handler/ | Gin route handlers |
| `S` | service.go | Use-case orchestration + business rules |
| `St` | store/ (Redis) | State persistence via `MigrationStore` port |
| `E` | execution/ + Temporal | Durable workflow engine via `ExecutionEngine` port |
| `M` | migrator/ | Outbound HTTP via `MigratorNotifier` / `DryRunner` ports |
| `W` | app-chart-migrator | External migrator service |

---

## 1. Announce

A migrator POSTs its migration definition and base URL on startup. After this the server knows which steps exist, who the candidates are, and the URL to dispatch steps to.

`migratorUrl` from the announcement is stored inside the `Migration` document in Redis and threaded through to every `DispatchStepRequest` — no separate in-memory registry is needed.

```mermaid
sequenceDiagram
    participant W as app-chart-migrator
    participant H as handler/
    participant S as service.go
    participant St as store/ (Redis)

    W->>H: POST /registry/announce {MigrationAnnouncement}
    H->>S: Announce(announcement)
    S->>St: Save(migration + candidates)
    St-->>S: ok
    S-->>H: migration
    H-->>W: 200 OK
```

---

## 2. Run lifecycle

The operator starts a candidate. The server validates, sets the candidate to `running`, starts a durable Temporal workflow, then immediately returns `202 Accepted`. Everything after that is asynchronous.

The workflow sequences each step in turn: it dispatches outbound to the migrator, then blocks waiting for a completion signal sent via the migrator's callback to `POST /event/:id`. Steps can go through intermediate states (`open`, `awaiting_review`) before reaching a terminal state (`completed`, `merged`, `failed`).

```mermaid
sequenceDiagram
    participant Con as Console
    participant H as handler/
    participant S as service.go
    participant St as store/ (Redis)
    participant E as Temporal (execution/)
    participant M as migrator/
    participant W as app-chart-migrator

    Con->>H: POST /migrations/:id/candidates/:cid/start
    H->>S: Start(migrationId, candidateId, inputs)
    S->>S: validate: migration + candidate exist, not already running
    S->>St: SetCandidateStatus(running)
    S->>E: StartRun(runType, runID, MigrationManifest)
    E-->>S: ok
    S-->>H: ok
    H-->>Con: 202 Accepted

    Note over E: MigrationOrchestrator begins

    loop for each step in manifest.Steps
        Note over E: upsert result → in_progress
        E->>M: DispatchStep activity → Dispatch(DispatchStepRequest)
        M->>W: POST /dispatch-step {DispatchStepRequest}
        W-->>M: 202 Accepted

        Note over W: executes step (e.g. opens PR on GitHub)

        opt PR-based step: open → merged
            W->>H: POST /event/:id {success: true, phase: "open"}
            H->>S: HandleEvent(runID, event)
            S->>E: RaiseEvent(runID, step-N-completed, event)
            Note over E: signal received, status=open — keep waiting
        end

        W->>H: POST /event/:id {StepCompletedEvent}
        H->>S: HandleEvent(runID, event)
        S->>E: RaiseEvent(runID, step-N-completed, event)
        H-->>W: 202 Accepted
        Note over E: terminal signal — advance to next step
    end

    E->>St: UpdateCandidateStatus activity → SetCandidateStatus(completed)
    St-->>E: ok
    Note over E: workflow complete
```

---

## 3. Dry run

The operator previews what a run would do for a specific candidate without making real changes. The call is fully synchronous — the console blocks until the migrator returns the diff.

```mermaid
sequenceDiagram
    participant Con as Console
    participant H as handler/
    participant S as service.go
    participant St as store/ (Redis)
    participant M as migrator/
    participant W as app-chart-migrator

    Con->>H: POST /migrations/:id/candidates/:cid/dry-run
    H->>S: DryRun(migrationId, candidateId)
    S->>St: Get(migration), GetCandidates(migrationId)
    St-->>S: migration + candidate
    S->>M: DryRun(migratorUrl, DryRunRequest)
    M->>W: POST /dry-run {DryRunRequest}
    W-->>M: 200 OK {DryRunResult}
    M-->>S: DryRunResult
    S-->>H: DryRunResult
    H-->>Con: 200 OK {DryRunResult}
```

---

## 4. Cancel

The operator cancels a running candidate. The server signals Temporal to cancel the workflow, then returns immediately. The workflow handles cancellation via a deferred cleanup that resets the candidate to `not_started` using a disconnected context — this ensures the cleanup activity runs even after the main workflow context is cancelled.

```mermaid
sequenceDiagram
    participant Con as Console
    participant H as handler/
    participant S as service.go
    participant E as Temporal (execution/)
    participant St as store/ (Redis)

    Con->>H: POST /migrations/:id/candidates/:cid/cancel
    H->>S: Cancel(migrationId, candidateId)
    S->>S: validate: candidate is running
    S->>E: CancelRun(runID)
    E-->>S: ok
    S-->>H: ok
    H-->>Con: 204 No Content

    Note over E: cancellation delivered to MigrationOrchestrator
    Note over E: deferred cleanup runs (disconnected context)

    E->>St: ResetCandidate activity → SetCandidateStatus(not_started)
    St-->>E: ok
    Note over E: workflow terminated
```

---

## 5. Step retry

A step has failed. The workflow blocks in `awaitRetryOrCancel`, waiting for either a retry signal or a cancel. The operator clicks Retry, which raises a signal into the running workflow. The workflow removes the failed result and re-dispatches the same step from scratch.

```mermaid
sequenceDiagram
    participant Con as Console
    participant H as handler/
    participant S as service.go
    participant E as Temporal (execution/)
    participant M as migrator/
    participant W as app-chart-migrator

    Note over E: workflow blocked — step failed, awaiting retry or cancel

    Con->>H: POST /migrations/:id/candidates/:cid/retry-step {stepName}
    H->>S: RetryStep(migrationId, candidateId, stepName)
    S->>S: validate: candidate is running
    S->>E: RaiseEvent(runID, retry-step-N-cid, nil)
    E-->>S: ok
    S-->>H: ok
    H-->>Con: 202 Accepted

    Note over E: retry signal received — remove failed result, re-dispatch

    E->>M: DispatchStep activity → Dispatch(DispatchStepRequest)
    M->>W: POST /dispatch-step {DispatchStepRequest}
    W-->>M: 202 Accepted
    Note over W: re-executes step
```
