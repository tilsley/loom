package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tilsley/loom/pkg/api"
)

// Service is the application-level use-case orchestrator for migrations.
// It depends only on port interfaces — no framework imports.
type Service struct {
	engine WorkflowEngine
	store  MigrationStore
}

// NewService creates a new Service.
func NewService(engine WorkflowEngine, store MigrationStore) *Service {
	return &Service{engine: engine, store: store}
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
	eventName := PROpenedEventName(event.StepName, event.Target)
	if err := s.engine.RaiseEvent(ctx, instanceID, eventName, event); err != nil {
		return fmt.Errorf("raise event %q: %w", eventName, err)
	}
	return nil
}

// HandleEvent raises a StepCompleted signal into the running workflow,
// unblocking the signal wait for the matching step+target.
func (s *Service) HandleEvent(ctx context.Context, instanceID string, event api.StepCompletedEvent) error {
	eventName := StepEventName(event.StepName, event.Target)
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
		existing.Targets = ann.Targets
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
		Targets:     ann.Targets,
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
		Targets:     req.Targets,
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

// Run creates a workflow instance from a registered migration for a single target.
func (s *Service) Run(ctx context.Context, id string, target api.Target) (string, error) {
	m, err := s.store.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get migration %q: %w", id, err)
	}
	if m == nil {
		return "", fmt.Errorf("migration %q not found", id)
	}

	// Guard: block re-running a target that is already running or completed.
	if m.TargetRuns != nil {
		if tr, ok := (*m.TargetRuns)[target.Repo]; ok {
			if tr.Status == api.TargetRunStatusRunning || tr.Status == api.TargetRunStatusCompleted {
				return "", TargetAlreadyRunError{Repo: target.Repo, Status: string(tr.Status)}
			}
		}
	}

	runID := GenerateRunID(m.Id)

	regID := m.Id
	manifest := api.MigrationManifest{
		MigrationId:    runID,
		RegistrationId: &regID,
		Targets:        []api.Target{target},
		Steps:          m.Steps,
	}

	if _, err := s.engine.StartWorkflow(ctx, "MigrationOrchestrator", runID, manifest); err != nil {
		return "", fmt.Errorf("start workflow: %w", err)
	}

	// Record the target as "running" and append the run ID.
	if err := s.store.SetTargetRun(
		ctx,
		id,
		target.Repo,
		api.TargetRun{RunId: runID, Status: api.TargetRunStatusRunning},
	); err != nil {
		return "", fmt.Errorf("set target run: %w", err)
	}
	if err := s.store.AppendRunID(ctx, id, runID); err != nil {
		return "", fmt.Errorf("append run ID: %w", err)
	}

	return runID, nil
}
