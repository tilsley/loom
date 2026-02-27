package migrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/tilsley/loom/pkg/api"
)

const instrName = "github.com/tilsley/loom"

// Service is the application-level use-case orchestrator for migrations.
// It depends only on port interfaces — no framework imports.
type Service struct {
	engine    ExecutionEngine
	store     MigrationStore
	dryRunner DryRunner

	// metrics
	runsStarted      metric.Int64Counter
	runsCancelled    metric.Int64Counter
	candidatesSubmit metric.Int64Counter
	dryRunsTotal     metric.Int64Counter
}

// NewService creates a new Service.
func NewService(engine ExecutionEngine, store MigrationStore, dryRunner DryRunner) *Service {
	m := otel.Meter(instrName)

	runsStarted, _ := m.Int64Counter("loom.runs.started",
		metric.WithDescription("Number of migration runs started"))
	runsCancelled, _ := m.Int64Counter("loom.runs.cancelled",
		metric.WithDescription("Number of migration runs cancelled"))
	candidatesSubmit, _ := m.Int64Counter("loom.candidates.submitted",
		metric.WithDescription("Number of candidates submitted"))
	dryRunsTotal, _ := m.Int64Counter("loom.dry_runs.total",
		metric.WithDescription("Number of dry runs executed"))

	return &Service{
		engine:           engine,
		store:            store,
		dryRunner:        dryRunner,
		runsStarted:      runsStarted,
		runsCancelled:    runsCancelled,
		candidatesSubmit: candidatesSubmit,
		dryRunsTotal:     dryRunsTotal,
	}
}

// GetCandidateSteps returns the step execution progress for a candidate's run.
// Returns nil (no error) when the run does not exist.
func (s *Service) GetCandidateSteps(ctx context.Context, migrationID, candidateID string) (*api.CandidateStepsResponse, error) {
	runID := RunID(migrationID, candidateID)
	ws, err := s.engine.GetStatus(ctx, runID)
	if err != nil {
		var notFound RunNotFoundError
		if errors.As(err, &notFound) {
			return nil, nil //nolint:nilnil
		}
		return nil, fmt.Errorf("get run status: %w", err)
	}

	steps := ws.Steps
	if steps == nil {
		steps = []api.StepState{}
	}

	// Derive response status from the engine's runtime status rather than parsing
	// the run output's status field. This correctly handles terminated and
	// cancelled runs, where ws.Output is nil and the output field would be empty.
	status := api.CandidateStepsResponseStatusRunning
	if ws.RuntimeStatus != RuntimeStatusRunning {
		status = api.CandidateStepsResponseStatusCompleted
	}

	return &api.CandidateStepsResponse{Status: status, Steps: steps}, nil
}

// HandleEvent raises a StepCompleted signal into the active run,
// unblocking the signal wait for the matching step+candidate.
func (s *Service) HandleEvent(ctx context.Context, instanceID string, event api.StepStatusEvent) error {
	eventName := StepEventName(event.StepName, event.CandidateId)
	if err := s.engine.RaiseEvent(ctx, instanceID, eventName, event); err != nil {
		return fmt.Errorf("raise event %q: %w", eventName, err)
	}
	return nil
}

// Announce upserts a migration from a migrator announcement (pub/sub discovery).
// The worker owns the ID (deterministic slug). Existing state and createdAt are preserved.
func (s *Service) Announce(ctx context.Context, ann api.MigrationAnnouncement) (*api.Migration, error) {
	existing, err := s.store.Get(ctx, ann.Id)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", ann.Id, err)
	}

	if existing != nil {
		// Upsert — update definition, preserve history and discovered candidates.
		existing.Name = ann.Name
		existing.Description = ann.Description
		existing.RequiredInputs = ann.RequiredInputs
		existing.Steps = ann.Steps
		existing.MigratorUrl = ann.MigratorUrl
		if err := s.store.Save(ctx, *existing); err != nil {
			return nil, fmt.Errorf("save migration: %w", err)
		}
		return existing, nil
	}

	m := api.Migration{
		Id:             ann.Id,
		Name:           ann.Name,
		Description:    ann.Description,
		RequiredInputs: ann.RequiredInputs,
		Candidates:     ann.Candidates,
		Steps:          ann.Steps,
		CreatedAt:      time.Now().UTC(),
		MigratorUrl:      ann.MigratorUrl,
	}
	if err := s.store.Save(ctx, m); err != nil {
		return nil, fmt.Errorf("save migration: %w", err)
	}
	return &m, nil
}

// List returns all migrations.
func (s *Service) List(ctx context.Context) ([]api.Migration, error) {
	migrations, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	return migrations, nil
}

// Get returns a specific migration by ID.
func (s *Service) Get(ctx context.Context, id string) (*api.Migration, error) {
	m, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", id, err)
	}
	return m, nil
}

// SubmitCandidates validates the migration exists, then persists the discovered candidate list.
func (s *Service) SubmitCandidates(ctx context.Context, migrationID string, req api.SubmitCandidatesRequest) error {
	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return MigrationNotFoundError{ID: migrationID}
	}
	if err := s.store.SaveCandidates(ctx, migrationID, req.Candidates); err != nil {
		return err
	}
	s.candidatesSubmit.Add(ctx, int64(len(req.Candidates)),
		metric.WithAttributes(attribute.String("migration_id", migrationID)))
	return nil
}

// GetCandidates returns the candidate list for a migration with their current status.
// Any candidate whose stored status is "running" but whose run no longer exists in
// the engine (e.g. after a Temporal restart) is automatically reset to "not_started".
func (s *Service) GetCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error) {
	candidates, err := s.store.GetCandidates(ctx, migrationID)
	if err != nil {
		return nil, err
	}

	for i, c := range candidates {
		if c.Status == nil || *c.Status != api.CandidateStatusRunning {
			continue
		}
		runID := RunID(migrationID, c.Id)
		if _, err := s.engine.GetStatus(ctx, runID); err != nil {
			var notFound RunNotFoundError
			if errors.As(err, &notFound) {
				// Stale run — reset to not_started so the Preview button becomes active again.
				_ = s.store.SetCandidateStatus(ctx, migrationID, c.Id, api.CandidateStatusNotStarted)
				notStarted := api.CandidateStatusNotStarted
				candidates[i].Status = &notStarted
			}
		}
	}

	return candidates, nil
}

// RetryStep raises a retry-step signal into the active run, re-dispatching the
// named step to the worker. Returns CandidateNotRunningError if the candidate is not running.
func (s *Service) RetryStep(ctx context.Context, migrationID, candidateID, stepName string) error {
	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return MigrationNotFoundError{ID: migrationID}
	}

	var found bool
	for _, c := range m.Candidates {
		if c.Id == candidateID {
			found = true
			if c.Status == nil || *c.Status != api.CandidateStatusRunning {
				return CandidateNotRunningError{ID: candidateID}
			}
			break
		}
	}
	if !found {
		return CandidateNotFoundError{MigrationID: migrationID, CandidateID: candidateID}
	}

	runID := RunID(migrationID, candidateID)
	eventName := RetryStepEventName(stepName, candidateID)
	if err := s.engine.RaiseEvent(ctx, runID, eventName, nil); err != nil {
		return fmt.Errorf("raise retry event: %w", err)
	}
	return nil
}

// Cancel stops the active run and resets the candidate to not_started so it can
// be previewed and started again.
func (s *Service) Cancel(ctx context.Context, migrationID, candidateID string) error {
	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return MigrationNotFoundError{ID: migrationID}
	}

	var found bool
	for _, c := range m.Candidates {
		if c.Id == candidateID {
			found = true
			if c.Status == nil || *c.Status != api.CandidateStatusRunning {
				return CandidateNotRunningError{ID: candidateID}
			}
			break
		}
	}
	if !found {
		return CandidateNotFoundError{MigrationID: migrationID, CandidateID: candidateID}
	}

	runID := RunID(migrationID, candidateID)

	if err := s.engine.CancelRun(ctx, runID); err != nil {
		var notFound RunNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("cancel run: %w", err)
		}
	}

	if err := s.store.SetCandidateStatus(ctx, migrationID, candidateID, api.CandidateStatusNotStarted); err != nil {
		return fmt.Errorf("reset candidate run: %w", err)
	}
	s.runsCancelled.Add(ctx, 1)
	return nil
}

// DryRun simulates a full migration run for a single candidate, returning
// per-step file diffs from the worker without creating any real PRs.
func (s *Service) DryRun(ctx context.Context, migrationID string, candidate api.Candidate) (*api.DryRunResult, error) {
	ctx, span := otel.Tracer(instrName).Start(ctx, "Service.DryRun",
		trace.WithAttributes(
			attribute.String("migration.id", migrationID),
			attribute.String("candidate.id", candidate.Id),
		),
	)
	defer span.End()

	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return nil, MigrationNotFoundError{ID: migrationID}
	}

	steps := m.Steps
	if candidate.Steps != nil && len(*candidate.Steps) > 0 {
		steps = *candidate.Steps
	}

	req := api.DryRunRequest{
		MigrationId: migrationID,
		Candidate:   candidate,
		Steps:       steps,
	}
	result, err := s.dryRunner.DryRun(ctx, m.MigratorUrl, req)
	status := "ok"
	if err != nil {
		status = "error"
		span.RecordError(err)
	}
	s.dryRunsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
	return result, err
}

// Start atomically looks up the candidate, merges any operator-supplied inputs,
// and starts a Run for the given migration+candidate pair.
func (s *Service) Start(ctx context.Context, migrationID, candidateID string, inputs map[string]string) (string, error) {
	ctx, span := otel.Tracer(instrName).Start(ctx, "Service.Start",
		trace.WithAttributes(
			attribute.String("migration.id", migrationID),
			attribute.String("candidate.id", candidateID),
		),
	)
	defer span.End()

	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return "", MigrationNotFoundError{ID: migrationID}
	}

	// Find the candidate in the migration's candidate list.
	var candidate api.Candidate
	found := false
	for _, c := range m.Candidates {
		if c.Id == candidateID {
			candidate = c
			found = true
			break
		}
	}
	if !found {
		return "", CandidateNotFoundError{MigrationID: migrationID, CandidateID: candidateID}
	}

	runID := RunID(migrationID, candidateID)

	// Guard: block if candidate is already running or completed AND the run
	// is still running or successfully completed. If the run has failed,
	// been cancelled, or terminated, allow re-execution even if Redis still
	// shows the old status (handles stale state after crash/cancel).
	if candidate.Status != nil &&
		(*candidate.Status == api.CandidateStatusRunning || *candidate.Status == api.CandidateStatusCompleted) {
		ws, err := s.engine.GetStatus(ctx, runID)
		if err == nil && (ws.RuntimeStatus == RuntimeStatusRunning || ws.RuntimeStatus == RuntimeStatusCompleted) {
			return "", CandidateAlreadyRunError{ID: candidateID, Status: string(*candidate.Status)}
		}
		// Run gone, failed, cancelled, or terminated — allow re-execution.
	}

	// Merge operator-supplied inputs into candidate metadata.
	if len(inputs) > 0 {
		if candidate.Metadata == nil {
			candidate.Metadata = &map[string]string{}
		}
		for k, v := range inputs {
			(*candidate.Metadata)[k] = v
		}
	}

	manifestSteps := m.Steps
	if candidate.Steps != nil && len(*candidate.Steps) > 0 {
		manifestSteps = *candidate.Steps
	}

	manifest := api.MigrationManifest{
		MigrationId: migrationID,
		Candidates:  []api.Candidate{candidate},
		Steps:       manifestSteps,
		MigratorUrl:   m.MigratorUrl,
	}

	if _, err := s.engine.StartRun(ctx, "MigrationOrchestrator", runID, manifest); err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("start run: %w", err)
	}

	if err := s.store.SetCandidateStatus(ctx, migrationID, candidateID, api.CandidateStatusRunning); err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("set candidate status: %w", err)
	}

	s.runsStarted.Add(ctx, 1,
		metric.WithAttributes(attribute.String("migration_id", migrationID)))
	return runID, nil
}
