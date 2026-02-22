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
	CandidateID  string `json:"candidateId"`
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
		return fmt.Errorf("dispatch step %q for %q: %w", req.StepName, req.Candidate.Id, err)
	}
	return nil
}

// CompensateStep is a placeholder for saga rollback. Real implementation
// (close PR, delete branch) will be added later.
func (a *Activities) CompensateStep(ctx context.Context, step api.StepResult) error {
	a.log.Info("compensating step",
		"step", step.StepName,
		"candidate", step.Candidate,
		"metadata", step.Metadata,
	)
	return nil
}

// ResetCandidateRunInput is the input for the ResetCandidateRun activity.
type ResetCandidateRunInput struct {
	RegistrationID string `json:"registrationId"`
	CandidateID    string `json:"candidateId"`
}

// ResetCandidateRun removes the candidate run entry, returning it to not_started state.
func (a *Activities) ResetCandidateRun(ctx context.Context, input ResetCandidateRunInput) error {
	if err := a.store.DeleteCandidateRun(ctx, input.RegistrationID, input.CandidateID); err != nil {
		return fmt.Errorf("reset candidate run: %w", err)
	}
	a.log.Info("reset candidate run to not_started",
		"registrationId", input.RegistrationID,
		"candidate", input.CandidateID,
	)
	return nil
}

// UpdateTargetRunStatus updates the candidate run status in the migration store.
func (a *Activities) UpdateTargetRunStatus(ctx context.Context, input UpdateTargetRunStatusInput) error {
	run := api.CandidateRun{
		RunId:  input.RunID,
		Status: api.CandidateRunStatus(input.Status),
	}
	if err := a.store.SetCandidateRun(ctx, input.RegistrationID, input.CandidateID, run); err != nil {
		return fmt.Errorf("update candidate run status: %w", err)
	}
	a.log.Info("updated candidate run status",
		"registrationId", input.RegistrationID,
		"candidate", input.CandidateID,
		"status", input.Status,
	)
	return nil
}
