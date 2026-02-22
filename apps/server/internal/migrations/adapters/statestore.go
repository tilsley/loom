package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	dapr "github.com/dapr/go-sdk/client"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

const (
	indexKey         = "migrations:index"
	storeKey         = "statestore"
	keyPrefix        = "migration:"
	candidatesPrefix = "migration-candidates:"
)

// Compile-time check: *DaprMigrationStore implements migrations.MigrationStore.
var _ migrations.MigrationStore = (*DaprMigrationStore)(nil)

// DaprMigrationStore implements MigrationStore using the Dapr state store.
type DaprMigrationStore struct {
	client dapr.Client
}

// NewDaprMigrationStore creates a new DaprMigrationStore.
func NewDaprMigrationStore(client dapr.Client) *DaprMigrationStore {
	return &DaprMigrationStore{client: client}
}

// Save persists a registered migration to the Dapr state store.
func (s *DaprMigrationStore) Save(ctx context.Context, m api.RegisteredMigration) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}

	// Save the migration itself.
	if err := s.client.SaveState(ctx, storeKey, keyPrefix+m.Id, data, nil); err != nil {
		return fmt.Errorf("save state %q: %w", m.Id, err)
	}

	// Append to the index only if not already present (idempotent for upserts).
	ids, err := s.loadIndex(ctx)
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}
	for _, existing := range ids {
		if existing == m.Id {
			return nil
		}
	}
	ids = append(ids, m.Id)
	return s.saveIndex(ctx, ids)
}

// Get retrieves a registered migration by ID, returning nil if not found.
func (s *DaprMigrationStore) Get(ctx context.Context, id string) (*api.RegisteredMigration, error) {
	item, err := s.client.GetState(ctx, storeKey, keyPrefix+id, nil)
	if err != nil {
		return nil, fmt.Errorf("get state %q: %w", id, err)
	}
	if len(item.Value) == 0 {
		return nil, nil //nolint:nilnil // caller checks nil value to detect "not found"
	}
	var m api.RegisteredMigration
	if err := json.Unmarshal(item.Value, &m); err != nil {
		return nil, fmt.Errorf("unmarshal migration %q: %w", id, err)
	}
	return &m, nil
}

// List returns all registered migrations.
func (s *DaprMigrationStore) List(ctx context.Context) ([]api.RegisteredMigration, error) {
	ids, err := s.loadIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	result := make([]api.RegisteredMigration, 0, len(ids))
	for _, id := range ids {
		m, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if m != nil {
			result = append(result, *m)
		}
	}
	return result, nil
}

// Delete removes a registered migration by ID.
func (s *DaprMigrationStore) Delete(ctx context.Context, id string) error {
	if err := s.client.DeleteState(ctx, storeKey, keyPrefix+id, nil); err != nil {
		return fmt.Errorf("delete state %q: %w", id, err)
	}

	// Remove from index.
	ids, err := s.loadIndex(ctx)
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}
	filtered := make([]string, 0, len(ids))
	for _, v := range ids {
		if v != id {
			filtered = append(filtered, v)
		}
	}
	return s.saveIndex(ctx, filtered)
}

// AppendCancelledAttempt records a cancelled run attempt on the migration.
func (s *DaprMigrationStore) AppendCancelledAttempt(ctx context.Context, migrationID string, attempt api.CancelledAttempt) error {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	if m.CancelledAttempts == nil {
		m.CancelledAttempts = &[]api.CancelledAttempt{}
	}
	*m.CancelledAttempts = append(*m.CancelledAttempts, attempt)

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}
	return s.client.SaveState(ctx, storeKey, keyPrefix+migrationID, data, nil)
}

// SetCandidateRun records the run status for a specific candidate within a migration.
func (s *DaprMigrationStore) SetCandidateRun(
	ctx context.Context,
	migrationID, candidateID string,
	run api.CandidateRun,
) error {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	if m.CandidateRuns == nil {
		m.CandidateRuns = &map[string]api.CandidateRun{}
	}
	(*m.CandidateRuns)[candidateID] = run

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}
	return s.client.SaveState(ctx, storeKey, keyPrefix+migrationID, data, nil)
}

// DeleteCandidateRun removes a candidate's run entry, returning it to not_started state.
func (s *DaprMigrationStore) DeleteCandidateRun(ctx context.Context, migrationID, candidateID string) error {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	if m.CandidateRuns == nil {
		return nil
	}
	delete(*m.CandidateRuns, candidateID)

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}
	return s.client.SaveState(ctx, storeKey, keyPrefix+migrationID, data, nil)
}

// SaveCandidates persists the discovered candidate list for a migration.
// It also updates the migration object's Candidates field so that List/Get
// returns the correct count without a separate lookup.
func (s *DaprMigrationStore) SaveCandidates(ctx context.Context, migrationID string, candidates []api.Candidate) error {
	data, err := json.Marshal(candidates)
	if err != nil {
		return fmt.Errorf("marshal candidates: %w", err)
	}
	if err := s.client.SaveState(ctx, storeKey, candidatesPrefix+migrationID, data, nil); err != nil {
		return fmt.Errorf("save candidates state %q: %w", migrationID, err)
	}

	// Keep the migration object in sync so List/Get returns the correct candidate count.
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m != nil {
		m.Candidates = candidates
		mData, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal migration: %w", err)
		}
		if err := s.client.SaveState(ctx, storeKey, keyPrefix+migrationID, mData, nil); err != nil {
			return fmt.Errorf("update migration candidates %q: %w", migrationID, err)
		}
	}
	return nil
}

// GetCandidates returns the candidate list for a migration, enriched with
// run status derived from the migration's targetRuns map.
func (s *DaprMigrationStore) GetCandidates(ctx context.Context, migrationID string) ([]api.CandidateWithStatus, error) {
	item, err := s.client.GetState(ctx, storeKey, candidatesPrefix+migrationID, nil)
	if err != nil {
		return nil, fmt.Errorf("get candidates state %q: %w", migrationID, err)
	}

	var candidates []api.Candidate
	if len(item.Value) > 0 {
		if err := json.Unmarshal(item.Value, &candidates); err != nil {
			return nil, fmt.Errorf("unmarshal candidates %q: %w", migrationID, err)
		}
	}

	// Load migration to access candidateRuns for status enrichment.
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", migrationID, err)
	}

	result := make([]api.CandidateWithStatus, 0, len(candidates))
	for _, c := range candidates {
		cs := api.CandidateWithStatus{
			Id:       c.Id,
			Kind:     c.Kind,
			Metadata: c.Metadata,
			State:    c.State,
			Files:    c.Files,
			Status:   api.CandidateStatusNotStarted,
		}
		if m != nil && m.CandidateRuns != nil {
			if cr, ok := (*m.CandidateRuns)[c.Id]; ok {
				runId := migrations.RunID(migrationID, c.Id)
				switch cr.Status {
				case api.CandidateRunStatusQueued:
					cs.Status = api.CandidateStatusQueued
					cs.RunId = &runId
				case api.CandidateRunStatusRunning:
					cs.Status = api.CandidateStatusRunning
					cs.RunId = &runId
				case api.CandidateRunStatusCompleted:
					cs.Status = api.CandidateStatusCompleted
					cs.RunId = &runId
				}
			}
		}
		result = append(result, cs)
	}

	return result, nil
}


// loadIndex reads the list of migration IDs from the index key.
func (s *DaprMigrationStore) loadIndex(ctx context.Context) ([]string, error) {
	item, err := s.client.GetState(ctx, storeKey, indexKey, nil)
	if err != nil {
		return nil, fmt.Errorf("get index: %w", err)
	}
	if len(item.Value) == 0 {
		return []string{}, nil
	}
	var ids []string
	if err := json.Unmarshal(item.Value, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}
	return ids, nil
}

// saveIndex writes the list of migration IDs to the index key.
func (s *DaprMigrationStore) saveIndex(ctx context.Context, ids []string) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	if err := s.client.SaveState(ctx, storeKey, indexKey, data, nil); err != nil {
		return fmt.Errorf("save index: %w", err)
	}
	return nil
}
