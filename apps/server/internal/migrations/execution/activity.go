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

// UpdateCandidateStatusInput is the input for the UpdateCandidateStatus activity.
type UpdateCandidateStatusInput struct {
	MigrationID string `json:"migrationId"`
	CandidateID string `json:"candidateId"`
	Status      string `json:"status"`
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
	a.log.Info("DispatchStep activity called", "step", req.StepName, "candidate", req.Candidate.Id, "workerUrl", req.WorkerUrl)

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

// ResetCandidateInput is the input for the ResetCandidate activity.
type ResetCandidateInput struct {
	MigrationID string `json:"migrationId"`
	CandidateID string `json:"candidateId"`
}

// ResetCandidate returns the candidate to not_started state.
func (a *Activities) ResetCandidate(ctx context.Context, input ResetCandidateInput) error {
	if err := a.store.SetCandidateStatus(ctx, input.MigrationID, input.CandidateID, api.CandidateStatusNotStarted); err != nil {
		return fmt.Errorf("reset candidate: %w", err)
	}
	a.log.Info("reset candidate to not_started",
		"migrationId", input.MigrationID,
		"candidate", input.CandidateID,
	)
	return nil
}

// UpdateCandidateStatus updates the candidate status in the migration store.
func (a *Activities) UpdateCandidateStatus(ctx context.Context, input UpdateCandidateStatusInput) error {
	ctx, span := otel.Tracer(instrName).Start(ctx, "UpdateCandidateStatus",
		trace.WithAttributes(
			attribute.String("candidate.id", input.CandidateID),
			attribute.String("status", input.Status),
		),
	)
	defer span.End()

	if err := a.store.SetCandidateStatus(ctx, input.MigrationID, input.CandidateID, api.CandidateStatus(input.Status)); err != nil {
		span.RecordError(err)
		return fmt.Errorf("update candidate status: %w", err)
	}
	a.log.Info("updated candidate status",
		"migrationId", input.MigrationID,
		"candidate", input.CandidateID,
		"status", input.Status,
	)
	return nil
}
