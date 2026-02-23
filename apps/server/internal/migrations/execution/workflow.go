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
// On failure, completed steps are compensated in reverse order (saga pattern).
//
//nolint:gocognit // orchestrator is inherently complex
func MigrationOrchestrator(
	ctx workflow.Context,
	manifest api.MigrationManifest,
) (MigrationResult, error) {
	results := make([]api.StepResult, 0, len(manifest.Steps)*len(manifest.Candidates))

	// Register query handler so external callers can read live progress.
	if err := workflow.SetQueryHandler(ctx, "progress", func() (MigrationResult, error) {
		return MigrationResult{
			MigrationId: manifest.MigrationId,
			Status:      statusRunning,
			Results:     results,
		}, nil
	}); err != nil {
		return MigrationResult{}, fmt.Errorf("register query handler: %w", err)
	}

	actOpts := workflow.ActivityOptions{
		TaskQueue:           workflow.GetInfo(ctx).TaskQueueName,
		StartToCloseTimeout: 24 * time.Hour,
	}
	actCtx := workflow.WithActivityOptions(ctx, actOpts)

	// updateCandidateStatus records the final status on the candidate in the store.
	updateCandidateStatus := func(status string) {
		if len(manifest.Candidates) == 0 {
			return
		}
		input := UpdateCandidateStatusInput{
			MigrationID: manifest.MigrationId,
			CandidateID: manifest.Candidates[0].Id,
			Status:      status,
		}
		fut := workflow.ExecuteActivity(actCtx, "UpdateCandidateStatus", input)
		if err := fut.Get(ctx, nil); err != nil {
			workflow.GetLogger(ctx).Warn("failed to update candidate status", "error", err, "status", status)
		}
	}

	// resetCandidate clears the candidate, returning it to not_started.
	// Used on failure — there is no "failed" status at the candidate level.
	resetCandidate := func() {
		if len(manifest.Candidates) == 0 {
			return
		}
		input := ResetCandidateInput{
			MigrationID: manifest.MigrationId,
			CandidateID: manifest.Candidates[0].Id,
		}
		fut := workflow.ExecuteActivity(actCtx, "ResetCandidate", input)
		if err := fut.Get(ctx, nil); err != nil {
			workflow.GetLogger(ctx).Warn("failed to reset candidate", "error", err)
		}
	}

	// Saga: on failure, compensate completed steps in reverse.
	var failed bool
	defer func() {
		if !failed {
			return
		}
		resetCandidate()
		// Use a disconnected context so compensation runs even if the workflow is cancelled.
		compensateAll(ctx, actOpts, results)
	}()

	for _, step := range manifest.Steps {
		for _, candidate := range manifest.Candidates {
			stepCompletedSignal := migrations.StepEventName(step.Name, candidate.Id)
			prOpenedSignal := migrations.PROpenedEventName(step.Name, candidate.Id)
			callbackID := workflow.GetInfo(ctx).WorkflowExecution.ID

			// 1. Manual-review steps skip worker dispatch; all others go to the worker.
			if step.Config != nil && (*step.Config)["type"] == "manual-review" {
				meta := map[string]string{"phase": "awaiting_review"}
				if instructions, ok := (*step.Config)["instructions"]; ok {
					meta["instructions"] = instructions
				}
				upsertResult(&results, api.StepResult{
					StepName:  step.Name,
					Candidate: candidate,
					Success:   true,
					Metadata:  &meta,
				})
			} else {
				req := api.DispatchStepRequest{
					MigrationId: manifest.MigrationId,
					StepName:    step.Name,
					Candidate:   candidate,
					Config:      step.Config,
					CallbackId:  callbackID,
					EventName:   stepCompletedSignal,
				}
				if err := workflow.ExecuteActivity(actCtx, "DispatchStep", req).Get(ctx, nil); err != nil {
					failed = true
					return MigrationResult{}, fmt.Errorf("dispatch step %q for %q: %w", step.Name, candidate.Id, err)
				}
			}

			// 2. Wait for signals. pr-opened is optional; step-completed ends the wait.
			prOpenedCh := workflow.GetSignalChannel(ctx, prOpenedSignal)
			stepCompletedCh := workflow.GetSignalChannel(ctx, stepCompletedSignal)

			done := false
			for !done {
				sel := workflow.NewSelector(ctx)

				sel.AddReceive(prOpenedCh, func(c workflow.ReceiveChannel, _ bool) {
					var event api.StepCompletedEvent
					c.Receive(ctx, &event)
					upsertResult(&results, api.StepResult{
						StepName:  event.StepName,
						Candidate: candidate,
						Success:   event.Success,
						Metadata:  event.Metadata,
					})
				})

				sel.AddReceive(stepCompletedCh, func(c workflow.ReceiveChannel, _ bool) {
					var event api.StepCompletedEvent
					c.Receive(ctx, &event)
					upsertResult(&results, api.StepResult{
						StepName:  event.StepName,
						Candidate: candidate,
						Success:   event.Success,
						Metadata:  event.Metadata,
					})
					done = true
				})

				sel.Select(ctx)
			}

			// 3. Check if the last completed step failed.
			last := results[len(results)-1]
			if !last.Success {
				failed = true
				return MigrationResult{
					MigrationId: manifest.MigrationId,
					Status:      statusFailed,
					Results:     results,
				}, nil
			}
		}
	}

	updateCandidateStatus(statusCompleted)

	return MigrationResult{
		MigrationId: manifest.MigrationId,
		Status:      statusCompleted,
		Results:     results,
	}, nil
}

// compensateAll runs CompensateStep for each completed result in reverse order.
// A disconnected context is used so compensation runs even if the workflow is cancelled.
func compensateAll(ctx workflow.Context, actOpts workflow.ActivityOptions, results []api.StepResult) {
	compCtx, _ := workflow.NewDisconnectedContext(ctx)
	compCtx = workflow.WithActivityOptions(compCtx, actOpts)
	for i := len(results) - 1; i >= 0; i-- {
		fut := workflow.ExecuteActivity(compCtx, "CompensateStep", results[i])
		if err := fut.Get(compCtx, nil); err != nil {
			workflow.GetLogger(compCtx).Warn("compensation step failed", "error", err, "step", results[i].StepName)
		}
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
