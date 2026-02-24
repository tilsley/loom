package execution

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// MigrationResult is the return type of MigrationOrchestrator.
// It is an internal Temporal type; the JSON structure is intentionally stable
// so that service.GetCandidateSteps can parse workflow output.
type MigrationResult struct {
	MigrationId string           `json:"migrationId"`
	Status      string           `json:"status"`
	Results     []api.StepResult `json:"results"`
}

const (
	statusRunning   = "running"
	statusCompleted = "completed"
	statusFailed    = "failed"
)

// MigrationOrchestrator is the Temporal workflow that sequences a full migration.
//
// For each step in the manifest it iterates candidate repos sequentially:
//  1. Dispatches the step to an external worker via the DispatchStep activity.
//  2. Listens for both "pr-opened" and "step-completed" signals via a Selector.
//  3. Records metadata (PR URLs, etc.) and advances to the next candidate/step.
//
// A query handler ("progress") exposes accumulated results in real-time.
func MigrationOrchestrator(
	ctx workflow.Context,
	manifest api.MigrationManifest,
) (MigrationResult, error) {
	results := make([]api.StepResult, 0, len(manifest.Steps)*len(manifest.Candidates))

	if err := workflow.SetQueryHandler(ctx, "progress", func() (MigrationResult, error) {
		return MigrationResult{
			MigrationId: manifest.MigrationId,
			Status:      statusRunning,
			Results:     results,
		}, nil
	}); err != nil {
		return MigrationResult{}, fmt.Errorf("register query handler: %w", err)
	}

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           workflow.GetInfo(ctx).TaskQueueName,
		StartToCloseTimeout: 24 * time.Hour,
	})

	var failed bool
	defer func() {
		if failed {
			// No saga compensation — PRs cannot be automatically undone.
			runResetCandidate(actCtx, ctx, manifest)
		}
	}()

	for _, step := range manifest.Steps {
		for _, candidate := range manifest.Candidates {
			ok, err := processStep(ctx, actCtx, manifest, step, candidate, &results)
			if err != nil {
				failed = true
				return MigrationResult{}, err
			}
			if !ok {
				failed = true
				return MigrationResult{
					MigrationId: manifest.MigrationId,
					Status:      statusFailed,
					Results:     results,
				}, nil
			}
		}
	}

	runUpdateCandidateStatus(actCtx, ctx, manifest, statusCompleted)

	return MigrationResult{
		MigrationId: manifest.MigrationId,
		Status:      statusCompleted,
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
	candidate api.Candidate,
	results *[]api.StepResult,
) (bool, error) {
	callbackID := workflow.GetInfo(ctx).WorkflowExecution.ID
	stepCompletedSignal := migrations.StepEventName(step.Name, candidate.Id)

	prOpenedCh := workflow.GetSignalChannel(ctx, migrations.PROpenedEventName(step.Name, candidate.Id))
	stepCompletedCh := workflow.GetSignalChannel(ctx, stepCompletedSignal)
	retryCh := workflow.GetSignalChannel(ctx, migrations.RetryStepEventName(step.Name, candidate.Id))

	for {
		if isManualReview(step) {
			upsertResult(results, manualReviewResult(step, candidate))
		} else {
			req := api.DispatchStepRequest{
				MigrationId: manifest.MigrationId,
				StepName:    step.Name,
				Candidate:   candidate,
				Config:      step.Config,
				CallbackId:  callbackID,
				EventName:   stepCompletedSignal,
				WorkerApp:   step.WorkerApp,
				WorkerUrl:   manifest.WorkerUrl,
			}
			if err := workflow.ExecuteActivity(actCtx, "DispatchStep", req).Get(ctx, nil); err != nil {
				return false, fmt.Errorf("dispatch step %q for %q: %w", step.Name, candidate.Id, err)
			}
		}

		awaitStepCompletion(ctx, prOpenedCh, stepCompletedCh, candidate, results)

		last := (*results)[len(*results)-1]
		if last.Success {
			return true, nil
		}

		removeResult(results, step.Name, candidate.Id)

		if !awaitRetryOrCancel(ctx, retryCh) {
			return false, nil // operator cancelled while waiting for retry
		}
		// retry signal received: loop back and re-dispatch the step
	}
}

// awaitStepCompletion blocks until a step-completed signal arrives, recording any
// pr-opened signals that arrive first (intermediate progress updates from the worker).
func awaitStepCompletion(
	ctx workflow.Context,
	prOpenedCh, stepCompletedCh workflow.ReceiveChannel,
	candidate api.Candidate,
	results *[]api.StepResult,
) {
	done := false
	for !done {
		sel := workflow.NewSelector(ctx)

		sel.AddReceive(prOpenedCh, func(c workflow.ReceiveChannel, _ bool) {
			var event api.StepCompletedEvent
			c.Receive(ctx, &event)
			upsertResult(results, api.StepResult{
				StepName:  event.StepName,
				Candidate: candidate,
				Success:   event.Success,
				Metadata:  event.Metadata,
			})
		})

		sel.AddReceive(stepCompletedCh, func(c workflow.ReceiveChannel, _ bool) {
			var event api.StepCompletedEvent
			c.Receive(ctx, &event)
			upsertResult(results, api.StepResult{
				StepName:  event.StepName,
				Candidate: candidate,
				Success:   event.Success,
				Metadata:  event.Metadata,
			})
			done = true
		})

		sel.Select(ctx)
	}
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

// isManualReview reports whether the step is a human gate rather than a worker dispatch.
func isManualReview(step api.StepDefinition) bool {
	return step.Config != nil && (*step.Config)["type"] == "manual-review"
}

// manualReviewResult builds the StepResult for a manual-review gate step.
func manualReviewResult(step api.StepDefinition, candidate api.Candidate) api.StepResult {
	meta := map[string]string{"phase": "awaiting_review"}
	if step.Config != nil {
		if instructions, ok := (*step.Config)["instructions"]; ok {
			meta["instructions"] = instructions
		}
	}
	return api.StepResult{
		StepName:  step.Name,
		Candidate: candidate,
		Success:   true,
		Metadata:  &meta,
	}
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
	if err := workflow.ExecuteActivity(actCtx, "UpdateCandidateStatus", input).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("failed to update candidate status", "error", err, "status", status)
	}
}

// runResetCandidate returns the candidate to not_started via the ResetCandidate activity.
// Called from a deferred function when the workflow fails or is cancelled mid-run.
func runResetCandidate(actCtx, ctx workflow.Context, manifest api.MigrationManifest) {
	if len(manifest.Candidates) == 0 {
		return
	}
	input := ResetCandidateInput{
		MigrationID: manifest.MigrationId,
		CandidateID: manifest.Candidates[0].Id,
	}
	if err := workflow.ExecuteActivity(actCtx, "ResetCandidate", input).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("failed to reset candidate", "error", err)
	}
}

// upsertResult updates an existing entry for the same step+candidate, or appends a new one.
func upsertResult(results *[]api.StepResult, r api.StepResult) {
	for i, existing := range *results {
		if existing.StepName == r.StepName && existing.Candidate.Id == r.Candidate.Id {
			(*results)[i] = r
			return
		}
	}
	*results = append(*results, r)
}

// removeResult removes the entry for the given step+candidate from results (inverse of upsertResult).
func removeResult(results *[]api.StepResult, stepName, candidateId string) {
	for i, r := range *results {
		if r.StepName == stepName && r.Candidate.Id == candidateId {
			*results = append((*results)[:i], (*results)[i+1:]...)
			return
		}
	}
}
