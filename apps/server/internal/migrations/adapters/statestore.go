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

// AppendRunID appends a run ID to the migration's run history.
func (s *DaprMigrationStore) AppendRunID(ctx context.Context, id, runID string) error {
	m, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", id)
	}
	m.RunIds = append(m.RunIds, runID)

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}
	return s.client.SaveState(ctx, storeKey, keyPrefix+id, data, nil)
}

// SetTargetRun records the run status for a specific target within a migration.
func (s *DaprMigrationStore) SetTargetRun(
	ctx context.Context,
	migrationID, targetRepo string,
	run api.TargetRun,
) error {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	if m.TargetRuns == nil {
		m.TargetRuns = &map[string]api.TargetRun{}
	}
	(*m.TargetRuns)[targetRepo] = run

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}
	return s.client.SaveState(ctx, storeKey, keyPrefix+migrationID, data, nil)
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
