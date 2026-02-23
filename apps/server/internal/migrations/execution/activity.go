package execution

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

const instrName = "github.com/tilsley/loom"

// UpdateTargetRunStatusInput is the input for the UpdateTargetRunStatus activity.
type UpdateTargetRunStatusInput struct {
	RegistrationID string `json:"registrationId"`
	CandidateID    string `json:"candidateId"`
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
	ctx, span := otel.Tracer(instrName).Start(ctx, "DispatchStep",
		trace.WithAttributes(
			attribute.String("step.name", req.StepName),
			attribute.String("candidate.id", req.Candidate.Id),
		),
	)
	defer span.End()

	if err := a.notifier.Dispatch(ctx, req); err != nil {
		span.RecordError(err)
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

// ResetCandidateRun returns the candidate to not_started state.
func (a *Activities) ResetCandidateRun(ctx context.Context, input ResetCandidateRunInput) error {
	if err := a.store.SetCandidateStatus(ctx, input.RegistrationID, input.CandidateID, api.CandidateStatusNotStarted); err != nil {
		return fmt.Errorf("reset candidate run: %w", err)
	}
	a.log.Info("reset candidate run to not_started",
		"registrationId", input.RegistrationID,
		"candidate", input.CandidateID,
	)
	return nil
}

// UpdateTargetRunStatus updates the candidate status in the migration store.
func (a *Activities) UpdateTargetRunStatus(ctx context.Context, input UpdateTargetRunStatusInput) error {
	ctx, span := otel.Tracer(instrName).Start(ctx, "UpdateTargetRunStatus",
		trace.WithAttributes(
			attribute.String("candidate.id", input.CandidateID),
			attribute.String("status", input.Status),
		),
	)
	defer span.End()

	if err := a.store.SetCandidateStatus(ctx, input.RegistrationID, input.CandidateID, api.CandidateStatus(input.Status)); err != nil {
		span.RecordError(err)
		return fmt.Errorf("update candidate run status: %w", err)
	}
	a.log.Info("updated candidate run status",
		"registrationId", input.RegistrationID,
		"candidate", input.CandidateID,
		"status", input.Status,
	)
	return nil
}
