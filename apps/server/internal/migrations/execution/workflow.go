package execution

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// MigrationOrchestrator is the Temporal workflow that sequences a full migration.
//
// For each step in the manifest it iterates target repos sequentially:
//  1. Dispatches the step to an external worker via the DispatchStep activity.
//  2. Listens for both "pr-opened" and "step-completed" signals via a Selector.
//  3. Records metadata (PR URLs, etc.) and advances to the next repo/step.
//
// A query handler ("progress") exposes accumulated results in real-time.
// On failure, completed steps are compensated in reverse order (saga pattern).
//
//nolint:gocognit // orchestrator is inherently complex
func MigrationOrchestrator(
	ctx workflow.Context,
	manifest api.MigrationManifest,
) (api.MigrationResult, error) {
	results := make([]api.StepResult, 0, len(manifest.Steps)*len(manifest.Targets))

	// Register query handler so external callers can read live progress.
	if err := workflow.SetQueryHandler(ctx, "progress", func() (api.MigrationResult, error) {
		return api.MigrationResult{
			MigrationId: manifest.MigrationId,
			Status:      api.MigrationResultStatusRunning,
			Results:     results,
		}, nil
	}); err != nil {
		return api.MigrationResult{}, fmt.Errorf("register query handler: %w", err)
	}

	actOpts := workflow.ActivityOptions{
		TaskQueue:           workflow.GetInfo(ctx).TaskQueueName,
		StartToCloseTimeout: 24 * time.Hour,
	}
	actCtx := workflow.WithActivityOptions(ctx, actOpts)

	// Helper: update the target run status in the registration store.
	// Skips if RegistrationId is nil (legacy /start path).
	updateTargetStatus := func(status string) {
		if manifest.RegistrationId == nil || len(manifest.Targets) == 0 {
			return
		}
		input := UpdateTargetRunStatusInput{
			RegistrationID: *manifest.RegistrationId,
			TargetRepo:     manifest.Targets[0].Repo,
			RunID:          manifest.MigrationId,
			Status:         status,
		}
		fut := workflow.ExecuteActivity(actCtx, "UpdateTargetRunStatus", input)
		if err := fut.Get(ctx, nil); err != nil {
			workflow.GetLogger(ctx).Warn("failed to update target run status", "error", err, "status", status)
		}
	}

	// Saga: on failure, compensate completed steps in reverse.
	var failed bool
	defer func() {
		if !failed {
			return
		}
		updateTargetStatus("failed")
		// Use a disconnected context so compensation runs even if the workflow is cancelled.
		compensateAll(ctx, actOpts, results)
	}()

	for _, step := range manifest.Steps {
		for _, target := range manifest.Targets {
			stepCompletedSignal := migrations.StepEventName(step.Name, target)
			prOpenedSignal := migrations.PROpenedEventName(step.Name, target)
			callbackID := workflow.GetInfo(ctx).WorkflowExecution.ID

			// 1. Manual-review steps skip worker dispatch; all others go to the worker.
			if step.Config != nil && (*step.Config)["type"] == "manual-review" {
				meta := map[string]string{"phase": "awaiting_review"}
				if instructions, ok := (*step.Config)["instructions"]; ok {
					meta["instructions"] = instructions
				}
				upsertResult(&results, api.StepResult{
					StepName: step.Name,
					Target:   target,
					Success:  true,
					Metadata: &meta,
				})
			} else {
				req := api.DispatchStepRequest{
					MigrationId: manifest.MigrationId,
					StepName:    step.Name,
					Target:      target,
					Config:      step.Config,
					CallbackId:  callbackID,
					EventName:   stepCompletedSignal,
				}
				if err := workflow.ExecuteActivity(actCtx, "DispatchStep", req).Get(ctx, nil); err != nil {
					failed = true
					return api.MigrationResult{}, fmt.Errorf("dispatch step %q for %q: %w", step.Name, target.Repo, err)
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
					upsertResult(&results, api.StepResult(event))
				})

				sel.AddReceive(stepCompletedCh, func(c workflow.ReceiveChannel, _ bool) {
					var event api.StepCompletedEvent
					c.Receive(ctx, &event)
					upsertResult(&results, api.StepResult(event))
					done = true
				})

				sel.Select(ctx)
			}

			// 3. Check if the last completed step failed.
			last := results[len(results)-1]
			if !last.Success {
				failed = true
				return api.MigrationResult{
					MigrationId: manifest.MigrationId,
					Status:      api.MigrationResultStatusFailed,
					Results:     results,
				}, nil
			}
		}
	}

	updateTargetStatus("completed")

	return api.MigrationResult{
		MigrationId: manifest.MigrationId,
		Status:      api.MigrationResultStatusCompleted,
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

// upsertResult updates an existing entry for the same step+target, or appends a new one.
func upsertResult(results *[]api.StepResult, r api.StepResult) {
	for i, existing := range *results {
		if existing.StepName == r.StepName && existing.Target.Repo == r.Target.Repo {
			(*results)[i] = r
			return
		}
	}
	*results = append(*results, r)
}
