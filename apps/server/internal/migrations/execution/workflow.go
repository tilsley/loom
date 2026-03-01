package execution

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// localActivityOptions for fire-and-forget event recording.
var localActOpts = workflow.LocalActivityOptions{
	ScheduleToCloseTimeout: 5 * time.Second,
}

// recordEvent fires a local activity to persist a lifecycle event.
// It is fire-and-forget: errors are logged but never block the workflow.
func recordEvent(ctx workflow.Context, event migrations.StepEvent) {
	laCtx := workflow.WithLocalActivityOptions(ctx, localActOpts)
	_ = workflow.ExecuteLocalActivity(laCtx, "RecordEvent", event).Get(laCtx, nil)
}

// MigrationResult is the return type of MigrationOrchestrator.
// It is an internal Temporal type; the JSON structure is intentionally stable
// so that service.GetCandidateSteps can parse workflow output.
type MigrationResult struct {
	MigrationId string          `json:"migrationId"`
	Status      string          `json:"status"`
	Results     []api.StepState `json:"results"`
}

const (
	resultRunning   = "running"
	resultCompleted = "completed"
	resultFailed    = "failed"
)

// MigrationOrchestrator is the Temporal workflow that sequences a full migration.
//
// For each step in the manifest it iterates candidate repos sequentially:
//  1. Dispatches the step to a migrator via the DispatchStep activity.
//  2. Waits for a "step-completed" signal from the migrator.
//  3. Records the result and advances to the next candidate/step.
//
// A query handler ("progress") exposes accumulated results in real-time.
func MigrationOrchestrator(
	ctx workflow.Context,
	manifest api.MigrationManifest,
) (MigrationResult, error) {
	workflow.GetLogger(ctx).Info("MigrationOrchestrator started", "migrationId", manifest.MigrationId, "steps", len(manifest.Steps), "candidates", len(manifest.Candidates))

	results := make([]api.StepState, 0, len(manifest.Steps)*len(manifest.Candidates))

	if err := workflow.SetQueryHandler(ctx, "progress", func() (MigrationResult, error) {
		return MigrationResult{
			MigrationId: manifest.MigrationId,
			Status:      resultRunning,
			Results:     results,
		}, nil
	}); err != nil {
		return MigrationResult{}, fmt.Errorf("register query handler: %w", err)
	}

	runStartTime := workflow.Now(ctx)

	// Record run_started event.
	recordEvent(ctx, migrations.StepEvent{
		MigrationID: manifest.MigrationId,
		CandidateID: candidateID(manifest),
		EventType:   migrations.EventRunStarted,
	})

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           workflow.GetInfo(ctx).TaskQueueName,
		StartToCloseTimeout: 24 * time.Hour,
	})

	var failed bool
	defer func() {
		if failed {
			// Record run_cancelled event with duration.
			dur := int(workflow.Now(ctx).Sub(runStartTime).Milliseconds())
			recordEvent(ctx, migrations.StepEvent{
				MigrationID: manifest.MigrationId,
				CandidateID: candidateID(manifest),
				EventType:   migrations.EventRunCancelled,
				DurationMs:  &dur,
			})

			// Use a disconnected context so the cleanup activity can run even if the
			// workflow context has been cancelled (e.g. operator hit Cancel).
			// Without this, ExecuteActivity on a cancelled ctx returns immediately
			// without scheduling, leaving the workflow stuck in RUNNING state forever.
			cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
			cleanupActCtx := workflow.WithActivityOptions(cleanupCtx, workflow.ActivityOptions{
				TaskQueue:           workflow.GetInfo(ctx).TaskQueueName,
				StartToCloseTimeout: 30 * time.Second,
			})
			// No saga compensation — PRs cannot be automatically undone.
			runResetCandidate(cleanupActCtx, cleanupCtx, manifest)
		}
	}()

	for _, step := range manifest.Steps {
		for i := range manifest.Candidates {
			ok, err := processStep(ctx, actCtx, manifest, step, &manifest.Candidates[i], &results)
			if err != nil {
				failed = true
				return MigrationResult{}, err
			}
			if !ok {
				failed = true
				return MigrationResult{
					MigrationId: manifest.MigrationId,
					Status:      resultFailed,
					Results:     results,
				}, nil
			}
		}
	}

	runUpdateCandidateStatus(actCtx, ctx, manifest, resultCompleted)

	// Record run_completed event with total duration.
	runDur := int(workflow.Now(ctx).Sub(runStartTime).Milliseconds())
	recordEvent(ctx, migrations.StepEvent{
		MigrationID: manifest.MigrationId,
		CandidateID: candidateID(manifest),
		EventType:   migrations.EventRunCompleted,
		DurationMs:  &runDur,
	})

	return MigrationResult{
		MigrationId: manifest.MigrationId,
		Status:      resultCompleted,
		Results:     results,
	}, nil
}

// processStep runs the retry loop for a single step+candidate pair.
// Returns (true, nil) on success, (false, nil) if the operator cancels while
// waiting for a retry, and (false, err) if the DispatchStep activity fails.
func processStep(
	ctx, actCtx workflow.Context,
	manifest api.MigrationManifest,
	step api.StepDefinition,
	candidate *api.Candidate,
	results *[]api.StepState,
) (bool, error) {
	callbackID := workflow.GetInfo(ctx).WorkflowExecution.ID
	stepCompletedSignal := migrations.StepEventName(step.Name, candidate.Id)

	stepCompletedCh := workflow.GetSignalChannel(ctx, stepCompletedSignal)
	retryCh := workflow.GetSignalChannel(ctx, migrations.RetryStepEventName(step.Name, candidate.Id))
	updateInputsCh := workflow.GetSignalChannel(ctx, migrations.UpdateInputsEventName(candidate.Id))

	for {
		// Drain any pending input updates before building the dispatch request
		// so that metadata edits made while the workflow was waiting take effect.
		drainInputUpdates(updateInputsCh, candidate)

		// Mark as in-progress before dispatching so the progress query
		// reflects the current step immediately — not only after it completes.
		upsertResult(results, api.StepState{
			StepName:  step.Name,
			Candidate: *candidate,
			Status:    api.StepStateStatusInProgress,
		})

		req := api.DispatchStepRequest{
			MigrationId: manifest.MigrationId,
			StepName:    step.Name,
			Candidate:   *candidate,
			Config:      step.Config,
			Type:        step.Type,
			CallbackId:  callbackID,
			EventName:   stepCompletedSignal,
			MigratorApp: step.MigratorApp,
			MigratorUrl: manifest.MigratorUrl,
		}
		stepStart := workflow.Now(ctx)
		if err := workflow.ExecuteActivity(actCtx, "DispatchStep", req).Get(ctx, nil); err != nil {
			return false, fmt.Errorf("dispatch step %q for %q: %w", step.Name, candidate.Id, err)
		}

		// Record step_dispatched.
		recordEvent(ctx, migrations.StepEvent{
			MigrationID: manifest.MigrationId,
			CandidateID: candidate.Id,
			StepName:    step.Name,
			EventType:   migrations.EventStepDispatched,
		})

		// Keep receiving signals until the step reaches a terminal state.
		// "pending" is intermediate — keep waiting for the final signal.
		// awaitStepCompletion returns false if the workflow was cancelled mid-wait,
		// in which case it does NOT append to results (safe to return immediately).
		// When it returns true it has always appended, so results is non-empty.
		for {
			if !awaitStepCompletion(ctx, stepCompletedCh, *candidate, results) {
				return false, nil // cancelled while waiting for step signal
			}
			last := (*results)[len(*results)-1]
			if last.Status != api.StepStateStatusPending {
				break
			}
		}

		last := (*results)[len(*results)-1]

		// Record step_completed with duration and status.
		stepDur := int(workflow.Now(ctx).Sub(stepStart).Milliseconds())
		stepStatus := string(last.Status)
		stepMeta := make(map[string]string)
		if last.Metadata != nil {
			for k, v := range *last.Metadata {
				stepMeta[k] = v
			}
		}
		recordEvent(ctx, migrations.StepEvent{
			MigrationID: manifest.MigrationId,
			CandidateID: candidate.Id,
			StepName:    step.Name,
			EventType:   migrations.EventStepCompleted,
			Status:      stepStatus,
			DurationMs:  &stepDur,
			Metadata:    stepMeta,
		})

		if last.Status == api.StepStateStatusSucceeded || last.Status == api.StepStateStatusMerged {
			return true, nil
		}

		// Leave the failed result visible in the query while waiting for retry/cancel
		// so the UI can show the failed state and retry button.
		if !awaitRetryOrCancel(ctx, retryCh) {
			return false, nil // operator cancelled while waiting for retry
		}

		// Record step_retried.
		recordEvent(ctx, migrations.StepEvent{
			MigrationID: manifest.MigrationId,
			CandidateID: candidate.Id,
			StepName:    step.Name,
			EventType:   migrations.EventStepRetried,
		})

		// Retry signal received: clear the failed result before re-dispatching.
		removeResult(results, step.Name, candidate.Id)
	}
}

// drainInputUpdates consumes all pending update-inputs signals from the channel
// and merges them into the candidate's metadata. ReceiveAsync is non-blocking —
// it returns false when the channel is empty.
func drainInputUpdates(ch workflow.ReceiveChannel, candidate *api.Candidate) {
	for {
		var inputs map[string]string
		if !ch.ReceiveAsync(&inputs) {
			return
		}
		if candidate.Metadata == nil {
			md := make(map[string]string)
			candidate.Metadata = &md
		}
		for k, v := range inputs {
			(*candidate.Metadata)[k] = v
		}
	}
}

// awaitStepCompletion blocks until a step-completed signal arrives or the
// workflow is cancelled. Returns false if the workflow was cancelled before
// a signal arrived (no result is appended in that case).
func awaitStepCompletion(
	ctx workflow.Context,
	stepCompletedCh workflow.ReceiveChannel,
	candidate api.Candidate,
	results *[]api.StepState,
) bool {
	var event api.StepStatusEvent
	var received bool
	sel := workflow.NewSelector(ctx)
	sel.AddReceive(stepCompletedCh, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, &event)
		received = true
	})
	sel.AddReceive(ctx.Done(), func(_ workflow.ReceiveChannel, _ bool) {})
	sel.Select(ctx)
	if !received {
		return false
	}

	upsertResult(results, api.StepState{
		StepName:  event.StepName,
		Candidate: candidate,
		Status:    api.StepStateStatus(event.Status),
		Metadata:  event.Metadata,
	})
	return true
}

// awaitRetryOrCancel blocks until either a retry-step signal or workflow cancellation.
// Returns true if a retry was requested, false if the workflow was cancelled.
func awaitRetryOrCancel(ctx workflow.Context, retryCh workflow.ReceiveChannel) bool {
	var retryReceived bool
	sel := workflow.NewSelector(ctx)
	sel.AddReceive(retryCh, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, nil)
		retryReceived = true
	})
	sel.AddReceive(ctx.Done(), func(_ workflow.ReceiveChannel, _ bool) {})
	sel.Select(ctx)
	return retryReceived
}

// runUpdateCandidateStatus persists the final candidate status via the
// UpdateCandidateStatus activity. Errors are logged but not propagated —
// a status update failure is not worth failing the workflow over.
func runUpdateCandidateStatus(actCtx, ctx workflow.Context, manifest api.MigrationManifest, status string) {
	if len(manifest.Candidates) == 0 {
		return
	}
	input := UpdateCandidateStatusInput{
		MigrationID: manifest.MigrationId,
		CandidateID: manifest.Candidates[0].Id,
		Status:      status,
	}
	// Temporal retries this activity with exponential backoff for up to the
	// StartToCloseTimeout (24h), so transient Redis failures self-heal.
	// If retries are exhausted (Redis down for >24h), we log and move on rather
	// than failing the workflow — the migration did complete, and propagating
	// this error would record it as failed in Temporal history, which is wrong.
	// The cost is a stale candidate status in Redis until the next run.
	if err := workflow.ExecuteActivity(actCtx, "UpdateCandidateStatus", input).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("failed to update candidate status", "error", err, "status", status)
	}
}

// runResetCandidate returns the candidate to not_started via the UpdateCandidateStatus activity.
// Called from a deferred function when the workflow fails or is cancelled mid-run.
func runResetCandidate(actCtx, ctx workflow.Context, manifest api.MigrationManifest) {
	if len(manifest.Candidates) == 0 {
		return
	}
	input := UpdateCandidateStatusInput{
		MigrationID: manifest.MigrationId,
		CandidateID: manifest.Candidates[0].Id,
		Status:      string(api.CandidateStatusNotStarted),
	}
	if err := workflow.ExecuteActivity(actCtx, "UpdateCandidateStatus", input).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("failed to reset candidate", "error", err)
	}
}

// upsertResult updates an existing entry for the same step+candidate, or appends a new one.
// When the incoming result has nil metadata, the existing metadata is preserved so that
// status transitions (e.g. pending→merged via the UI) don't discard worker-provided
// metadata such as prUrl.
func upsertResult(results *[]api.StepState, r api.StepState) {
	for i, existing := range *results {
		if existing.StepName == r.StepName && existing.Candidate.Id == r.Candidate.Id {
			if r.Metadata == nil {
				r.Metadata = existing.Metadata
			}
			(*results)[i] = r
			return
		}
	}
	*results = append(*results, r)
}

// removeResult removes the entry for the given step+candidate from results (inverse of upsertResult).
func removeResult(results *[]api.StepState, stepName, candidateId string) {
	for i, r := range *results {
		if r.StepName == stepName && r.Candidate.Id == candidateId {
			*results = append((*results)[:i], (*results)[i+1:]...)
			return
		}
	}
}

// candidateID returns the first candidate ID from the manifest, or empty string.
func candidateID(manifest api.MigrationManifest) string {
	if len(manifest.Candidates) > 0 {
		return manifest.Candidates[0].Id
	}
	return ""
}
