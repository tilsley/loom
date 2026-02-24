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
	indexKey  = "migrations:index"
	storeKey  = "statestore"
	keyPrefix = "migration:"
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

// Save persists a migration to the Dapr state store.
func (s *DaprMigrationStore) Save(ctx context.Context, m api.Migration) error {
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

// Get retrieves a migration by ID, returning nil if not found.
func (s *DaprMigrationStore) Get(ctx context.Context, id string) (*api.Migration, error) {
	item, err := s.client.GetState(ctx, storeKey, keyPrefix+id, nil)
	if err != nil {
		return nil, fmt.Errorf("get state %q: %w", id, err)
	}
	if len(item.Value) == 0 {
		return nil, nil //nolint:nilnil // caller checks nil value to detect "not found"
	}
	var m api.Migration
	if err := json.Unmarshal(item.Value, &m); err != nil {
		return nil, fmt.Errorf("unmarshal migration %q: %w", id, err)
	}
	return &m, nil
}

// List returns all migrations.
func (s *DaprMigrationStore) List(ctx context.Context) ([]api.Migration, error) {
	ids, err := s.loadIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	result := make([]api.Migration, 0, len(ids))
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

// SetCandidateStatus updates the status of a specific candidate within a migration.
func (s *DaprMigrationStore) SetCandidateStatus(
	ctx context.Context,
	migrationID, candidateID string,
	status api.CandidateStatus,
) error {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	for i, c := range m.Candidates {
		if c.Id == candidateID {
			m.Candidates[i].Status = &status
			data, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("marshal migration: %w", err)
			}
			return s.client.SaveState(ctx, storeKey, keyPrefix+migrationID, data, nil)
		}
	}
	return fmt.Errorf("candidate %q not found in migration %q", candidateID, migrationID)
}

// SaveCandidates merges the discovered candidate list into the migration.
// Candidates already in running or completed state are preserved as-is.
// Running/completed candidates not in the incoming list are also preserved.
func (s *DaprMigrationStore) SaveCandidates(ctx context.Context, migrationID string, incoming []api.Candidate) error {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}

	// Build a map of existing candidates by ID to check their status.
	existing := make(map[string]api.Candidate, len(m.Candidates))
	for _, c := range m.Candidates {
		existing[c.Id] = c
	}

	notStarted := api.CandidateStatusNotStarted
	incomingIDs := make(map[string]bool, len(incoming))
	merged := make([]api.Candidate, 0, len(incoming))
	for _, c := range incoming {
		incomingIDs[c.Id] = true
		if ex, ok := existing[c.Id]; ok && ex.Status != nil &&
			(*ex.Status == api.CandidateStatusRunning || *ex.Status == api.CandidateStatusCompleted) {
			// Preserve running/completed candidate state.
			merged = append(merged, ex)
		} else {
			// New or not-started candidate: add with not_started status.
			c.Status = &notStarted
			merged = append(merged, c)
		}
	}

	// Keep running/completed candidates that are no longer in the incoming list.
	for _, ex := range m.Candidates {
		if !incomingIDs[ex.Id] && ex.Status != nil &&
			(*ex.Status == api.CandidateStatusRunning || *ex.Status == api.CandidateStatusCompleted) {
			merged = append(merged, ex)
		}
	}

	m.Candidates = merged
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}
	return s.client.SaveState(ctx, storeKey, keyPrefix+migrationID, data, nil)
}

// GetCandidates returns the candidate list for a migration with their current status.
func (s *DaprMigrationStore) GetCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error) {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return nil, nil //nolint:nilnil // migration not found treated as no candidates
	}
	return m.Candidates, nil
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
