package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tilsley/loom/pkg/api"
)

// Service is the application-level use-case orchestrator for migrations.
// It depends only on port interfaces — no framework imports.
type Service struct {
	engine    WorkflowEngine
	store     MigrationStore
	dryRunner DryRunner
}

// NewService creates a new Service.
func NewService(engine WorkflowEngine, store MigrationStore, dryRunner DryRunner) *Service {
	return &Service{engine: engine, store: store, dryRunner: dryRunner}
}

// Start schedules a new migration workflow from the given manifest.
func (s *Service) Start(ctx context.Context, manifest api.MigrationManifest) (string, error) {
	id, err := s.engine.StartWorkflow(ctx, "MigrationOrchestrator", manifest.MigrationId, manifest)
	if err != nil {
		return "", fmt.Errorf("start workflow: %w", err)
	}
	return id, nil
}

// Status returns the current state of a migration, including accumulated
// PR links and step metadata. For running workflows, the Temporal engine
// adapter returns live progress via the query handler directly in ws.Output.
func (s *Service) Status(ctx context.Context, instanceID string) (*MigrationStatus, error) {
	ws, err := s.engine.GetStatus(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get workflow status: %w", err)
	}
	ms := &MigrationStatus{
		InstanceID:    instanceID,
		RuntimeStatus: ws.RuntimeStatus,
	}
	if len(ws.Output) > 0 {
		var result api.MigrationResult
		if err := json.Unmarshal(ws.Output, &result); err == nil {
			ms.Result = &result
		}
	}
	return ms, nil
}

// HandlePROpened signals the running workflow with a pr-opened event so the
// query handler can expose intermediate PR URLs in real-time.
func (s *Service) HandlePROpened(ctx context.Context, instanceID string, event api.StepCompletedEvent) error {
	eventName := PROpenedEventName(event.StepName, event.Candidate)
	if err := s.engine.RaiseEvent(ctx, instanceID, eventName, event); err != nil {
		return fmt.Errorf("raise event %q: %w", eventName, err)
	}
	return nil
}

// HandleEvent raises a StepCompleted signal into the running workflow,
// unblocking the signal wait for the matching step+candidate.
func (s *Service) HandleEvent(ctx context.Context, instanceID string, event api.StepCompletedEvent) error {
	eventName := StepEventName(event.StepName, event.Candidate)
	if err := s.engine.RaiseEvent(ctx, instanceID, eventName, event); err != nil {
		return fmt.Errorf("raise event %q: %w", eventName, err)
	}
	return nil
}

// Announce upserts a migration from a worker announcement (pub/sub discovery).
// The worker owns the ID (deterministic slug). Existing runIds and createdAt are preserved.
func (s *Service) Announce(ctx context.Context, ann api.MigrationAnnouncement) (*api.RegisteredMigration, error) {
	existing, err := s.store.Get(ctx, ann.Id)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", ann.Id, err)
	}

	if existing != nil {
		// Upsert — update definition, preserve history.
		existing.Name = ann.Name
		existing.Description = ann.Description
		existing.Candidates = ann.Candidates
		existing.Steps = ann.Steps
		if err := s.store.Save(ctx, *existing); err != nil {
			return nil, fmt.Errorf("save migration: %w", err)
		}
		return existing, nil
	}

	m := api.RegisteredMigration{
		Id:          ann.Id,
		Name:        ann.Name,
		Description: ann.Description,
		Candidates:  ann.Candidates,
		Steps:       ann.Steps,
		CreatedAt:   time.Now().UTC(),
		RunIds:      []string{},
	}
	if err := s.store.Save(ctx, m); err != nil {
		return nil, fmt.Errorf("save migration: %w", err)
	}
	return &m, nil
}

// Register persists a new migration definition and returns it with a generated ID.
func (s *Service) Register(ctx context.Context, req api.RegisterMigrationRequest) (*api.RegisteredMigration, error) {
	m := api.RegisteredMigration{
		Id:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Candidates:  req.Candidates,
		Steps:       req.Steps,
		CreatedAt:   time.Now().UTC(),
		RunIds:      []string{},
	}
	if err := s.store.Save(ctx, m); err != nil {
		return nil, fmt.Errorf("save migration: %w", err)
	}
	return &m, nil
}

// List returns all registered migrations.
func (s *Service) List(ctx context.Context) ([]api.RegisteredMigration, error) {
	migrations, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	return migrations, nil
}

// Get returns a specific registered migration by ID.
func (s *Service) Get(ctx context.Context, id string) (*api.RegisteredMigration, error) {
	m, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", id, err)
	}
	return m, nil
}

// DeleteMigration removes a registered migration by ID.
func (s *Service) DeleteMigration(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete migration %q: %w", id, err)
	}
	return nil
}

// SubmitCandidates validates the migration exists, then persists the discovered candidate list.
func (s *Service) SubmitCandidates(ctx context.Context, migrationID string, req api.SubmitCandidatesRequest) error {
	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	return s.store.SaveCandidates(ctx, migrationID, req.Candidates)
}

// GetCandidates returns the candidate list for a migration, enriched with run status.
// Any candidate whose stored status is "running" but whose workflow no longer exists in
// the engine (e.g. after a Temporal restart) is automatically reset to "not_started" so the
// candidate becomes re-runnable without manual intervention.
// Candidates with status "queued" are skipped — no workflow exists yet for them.
func (s *Service) GetCandidates(ctx context.Context, migrationID string) ([]api.CandidateWithStatus, error) {
	candidates, err := s.store.GetCandidates(ctx, migrationID)
	if err != nil {
		return nil, err
	}

	for i, c := range candidates {
		if c.Status != api.CandidateStatusRunning || c.RunId == nil {
			continue
		}
		if _, err := s.engine.GetStatus(ctx, *c.RunId); err != nil {
			var notFound WorkflowNotFoundError
			if errors.As(err, &notFound) {
				// Stale workflow — reset to not_started so the Queue button becomes active again.
				_ = s.store.DeleteCandidateRun(ctx, migrationID, c.Id)
				candidates[i].Status = api.CandidateStatusNotStarted
				candidates[i].RunId = nil
			}
		}
	}

	return candidates, nil
}

// GetRunInfo returns metadata about a run — including queued runs that have no workflow yet.
func (s *Service) GetRunInfo(ctx context.Context, runID string) (*api.RunInfo, error) {
	record, err := s.store.GetRunRecord(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get run record %q: %w", runID, err)
	}
	if record == nil {
		return nil, nil //nolint:nilnil // caller checks nil to detect "not found"
	}

	m, err := s.store.Get(ctx, record.MigrationID)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", record.MigrationID, err)
	}

	// If the migration or candidateRun entry no longer exists the run was dequeued.
	if m == nil || m.CandidateRuns == nil {
		return nil, nil //nolint:nilnil
	}
	cr, ok := (*m.CandidateRuns)[record.Candidate.Id]
	if !ok {
		return nil, nil //nolint:nilnil
	}

	var status api.RunInfoStatus
	switch cr.Status {
	case api.CandidateRunStatusRunning:
		status = api.RunInfoStatusRunning
	case api.CandidateRunStatusCompleted:
		status = api.RunInfoStatusCompleted
	default:
		status = api.RunInfoStatusQueued
	}

	return &api.RunInfo{
		RunId:       runID,
		MigrationId: record.MigrationID,
		Candidate:   record.Candidate,
		Status:      status,
	}, nil
}

// Cancel stops a running workflow, records a CancelledAttempt audit entry, and resets
// the candidate to not_started so it can be re-queued or previewed again.
func (s *Service) Cancel(ctx context.Context, runID string) error {
	record, err := s.store.GetRunRecord(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run record %q: %w", runID, err)
	}
	if record == nil {
		return fmt.Errorf("run %q not found", runID)
	}

	if err := s.engine.CancelWorkflow(ctx, runID); err != nil {
		var notFound WorkflowNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("cancel workflow: %w", err)
		}
	}

	attempt := api.CancelledAttempt{
		RunId:       runID,
		CandidateId: record.Candidate.Id,
		CancelledAt: time.Now().UTC(),
	}
	if err := s.store.AppendCancelledAttempt(ctx, record.MigrationID, attempt); err != nil {
		return fmt.Errorf("record cancelled attempt: %w", err)
	}
	if err := s.store.DeleteCandidateRun(ctx, record.MigrationID, record.Candidate.Id); err != nil {
		return fmt.Errorf("reset candidate run: %w", err)
	}
	if err := s.store.DeleteRunRecord(ctx, runID); err != nil {
		return fmt.Errorf("delete run record: %w", err)
	}
	return nil
}

// Dequeue removes a run from the queue, returning the candidate to not_started state.
func (s *Service) Dequeue(ctx context.Context, runID string) error {
	record, err := s.store.GetRunRecord(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run record %q: %w", runID, err)
	}
	if record == nil {
		return fmt.Errorf("run %q not found", runID)
	}

	m, err := s.store.Get(ctx, record.MigrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", record.MigrationID, err)
	}
	if m != nil && m.CandidateRuns != nil {
		if cr, ok := (*m.CandidateRuns)[record.Candidate.Id]; ok && cr.Status != api.CandidateRunStatusQueued {
			return CandidateAlreadyRunError{ID: record.Candidate.Id, Status: string(cr.Status)}
		}
	}

	if err := s.store.DeleteCandidateRun(ctx, record.MigrationID, record.Candidate.Id); err != nil {
		return fmt.Errorf("delete candidate run: %w", err)
	}
	if err := s.store.DeleteRunRecord(ctx, runID); err != nil {
		return fmt.Errorf("delete run record: %w", err)
	}
	return nil
}

// Queue reserves a run for a single candidate without starting the workflow.
// The run ID is stored so Execute can retrieve it later.
func (s *Service) Queue(ctx context.Context, migrationID string, candidate api.Candidate) (string, error) {
	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		return "", fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return "", fmt.Errorf("migration %q not found", migrationID)
	}

	// Guard: block if candidate is already queued, running, or completed.
	if m.CandidateRuns != nil {
		if cr, ok := (*m.CandidateRuns)[candidate.Id]; ok {
			if cr.Status == api.CandidateRunStatusQueued || cr.Status == api.CandidateRunStatusRunning || cr.Status == api.CandidateRunStatusCompleted {
				// For running/completed, check whether the workflow still exists.
				if cr.Status == api.CandidateRunStatusRunning || cr.Status == api.CandidateRunStatusCompleted {
					if _, err := s.engine.GetStatus(ctx, cr.RunId); err == nil {
						return "", CandidateAlreadyRunError{ID: candidate.Id, Status: string(cr.Status)}
					}
					// Workflow gone — fall through to allow re-queueing.
				} else {
					return "", CandidateAlreadyRunError{ID: candidate.Id, Status: string(cr.Status)}
				}
			}
		}
	}

	runID := GenerateRunID(m.Id)

	if err := s.store.StoreRunRecord(ctx, runID, RunRecord{MigrationID: migrationID, Candidate: candidate}); err != nil {
		return "", fmt.Errorf("store run record: %w", err)
	}
	if err := s.store.SetCandidateRun(ctx, migrationID, candidate.Id, api.CandidateRun{RunId: runID, Status: api.CandidateRunStatusQueued}); err != nil {
		return "", fmt.Errorf("set candidate run: %w", err)
	}
	if err := s.store.AppendRunID(ctx, migrationID, runID); err != nil {
		return "", fmt.Errorf("append run ID: %w", err)
	}

	return runID, nil
}

// DryRun simulates a full migration run for a single candidate, returning
// per-step file diffs from the worker without creating any real PRs.
func (s *Service) DryRun(ctx context.Context, migrationID string, candidate api.Candidate) (*api.DryRunResult, error) {
	m, err := s.store.Get(ctx, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return nil, fmt.Errorf("migration %q not found", migrationID)
	}

	req := api.DryRunRequest{
		MigrationId: migrationID,
		Candidate:   candidate,
		Steps:       m.Steps,
	}
	return s.dryRunner.DryRun(ctx, req)
}

// Execute starts the Temporal workflow for a previously queued run.
func (s *Service) Execute(ctx context.Context, runID string) (string, error) {
	record, err := s.store.GetRunRecord(ctx, runID)
	if err != nil {
		return "", fmt.Errorf("get run record %q: %w", runID, err)
	}
	if record == nil {
		return "", fmt.Errorf("run %q not found", runID)
	}

	m, err := s.store.Get(ctx, record.MigrationID)
	if err != nil {
		return "", fmt.Errorf("get migration %q: %w", record.MigrationID, err)
	}
	if m == nil {
		return "", fmt.Errorf("migration %q not found", record.MigrationID)
	}

	// Guard: run must be in queued state.
	if m.CandidateRuns != nil {
		if cr, ok := (*m.CandidateRuns)[record.Candidate.Id]; ok {
			if cr.Status == api.CandidateRunStatusRunning || cr.Status == api.CandidateRunStatusCompleted {
				return "", CandidateAlreadyRunError{ID: record.Candidate.Id, Status: string(cr.Status)}
			}
		}
	}

	regID := m.Id
	manifest := api.MigrationManifest{
		MigrationId:    runID,
		RegistrationId: &regID,
		Candidates:     []api.Candidate{record.Candidate},
		Steps:          m.Steps,
	}

	if _, err := s.engine.StartWorkflow(ctx, "MigrationOrchestrator", runID, manifest); err != nil {
		return "", fmt.Errorf("start workflow: %w", err)
	}

	if err := s.store.SetCandidateRun(ctx, record.MigrationID, record.Candidate.Id, api.CandidateRun{RunId: runID, Status: api.CandidateRunStatusRunning}); err != nil {
		return "", fmt.Errorf("set candidate run: %w", err)
	}

	return runID, nil
}
