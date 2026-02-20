package execution

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// UpdateTargetRunStatusInput is the input for the UpdateTargetRunStatus activity.
type UpdateTargetRunStatusInput struct {
	RegistrationID string `json:"registrationId"`
	TargetRepo     string `json:"targetRepo"`
	RunID          string `json:"runId"`
	Status         string `json:"status"`
}

// Activities groups Temporal activity methods. The struct holds dependencies
// injected at startup (idiomatic Temporal pattern).
type Activities struct {
	notifier migrations.WorkerNotifier
	store    migrations.MigrationStore
	log      *slog.Logger
}

// NewActivities creates a new Activities instance with the given dependencies.
func NewActivities(notifier migrations.WorkerNotifier, store migrations.MigrationStore, log *slog.Logger) *Activities {
	return &Activities{notifier: notifier, store: store, log: log}
}

// DispatchStep publishes a step request to the external worker via WorkerNotifier.
func (a *Activities) DispatchStep(ctx context.Context, req api.DispatchStepRequest) error {
	if err := a.notifier.Dispatch(ctx, req); err != nil {
		return fmt.Errorf("dispatch step %q for %q: %w", req.StepName, req.Target.Repo, err)
	}
	return nil
}

// CompensateStep is a placeholder for saga rollback. Real implementation
// (close PR, delete branch) will be added later.
func (a *Activities) CompensateStep(ctx context.Context, step api.StepResult) error {
	a.log.Info("compensating step",
		"step", step.StepName,
		"target", step.Target,
		"metadata", step.Metadata,
	)
	return nil
}

// UpdateTargetRunStatus updates the target run status in the migration store.
func (a *Activities) UpdateTargetRunStatus(ctx context.Context, input UpdateTargetRunStatusInput) error {
	run := api.TargetRun{
		RunId:  input.RunID,
		Status: api.TargetRunStatus(input.Status),
	}
	if err := a.store.SetTargetRun(ctx, input.RegistrationID, input.TargetRepo, run); err != nil {
		return fmt.Errorf("update target run status: %w", err)
	}
	a.log.Info("updated target run status",
		"registrationId", input.RegistrationID,
		"target", input.TargetRepo,
		"status", input.Status,
	)
	return nil
}
