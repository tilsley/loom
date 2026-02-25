package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

const (
	redisIndexKey  = "migrations:index"
	redisKeyPrefix = "migration:"
)

// Compile-time check: *RedisMigrationStore implements migrations.MigrationStore.
var _ migrations.MigrationStore = (*RedisMigrationStore)(nil)

// RedisMigrationStore implements MigrationStore using go-redis directly.
type RedisMigrationStore struct {
	rdb *redis.Client
}

// NewRedisMigrationStore creates a new RedisMigrationStore.
func NewRedisMigrationStore(rdb *redis.Client) *RedisMigrationStore {
	return &RedisMigrationStore{rdb: rdb}
}

// Save persists a migration and adds its ID to the index set.
func (s *RedisMigrationStore) Save(ctx context.Context, m api.Migration) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration: %w", err)
	}
	if err := s.rdb.Set(ctx, redisKeyPrefix+m.Id, data, 0).Err(); err != nil {
		return fmt.Errorf("save migration %q: %w", m.Id, err)
	}
	// SADD is idempotent — safe to call even if ID is already in the set.
	if err := s.rdb.SAdd(ctx, redisIndexKey, m.Id).Err(); err != nil {
		return fmt.Errorf("update index for %q: %w", m.Id, err)
	}
	return nil
}

// Get retrieves a migration by ID, returning nil if not found.
func (s *RedisMigrationStore) Get(ctx context.Context, id string) (*api.Migration, error) {
	val, err := s.rdb.Get(ctx, redisKeyPrefix+id).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil //nolint:nilnil // caller checks nil value to detect "not found"
	}
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", id, err)
	}
	var m api.Migration
	if err := json.Unmarshal([]byte(val), &m); err != nil {
		return nil, fmt.Errorf("unmarshal migration %q: %w", id, err)
	}
	return &m, nil
}

// List returns all migrations.
func (s *RedisMigrationStore) List(ctx context.Context) ([]api.Migration, error) {
	ids, err := s.rdb.SMembers(ctx, redisIndexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("list index: %w", err)
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
func (s *RedisMigrationStore) SetCandidateStatus(
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
			return s.rdb.Set(ctx, redisKeyPrefix+migrationID, data, 0).Err()
		}
	}
	return fmt.Errorf("candidate %q not found in migration %q", candidateID, migrationID)
}

// SaveCandidates merges the discovered candidate list into the migration.
// Candidates already in running or completed state are preserved as-is.
func (s *RedisMigrationStore) SaveCandidates(ctx context.Context, migrationID string, incoming []api.Candidate) error {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return fmt.Errorf("migration %q not found", migrationID)
	}

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
			merged = append(merged, ex)
		} else {
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
	return s.rdb.Set(ctx, redisKeyPrefix+migrationID, data, 0).Err()
}

// GetCandidates returns the candidate list for a migration with their current status.
func (s *RedisMigrationStore) GetCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error) {
	m, err := s.Get(ctx, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get migration %q: %w", migrationID, err)
	}
	if m == nil {
		return nil, nil //nolint:nilnil // migration not found treated as no candidates
	}
	return m.Candidates, nil
}

