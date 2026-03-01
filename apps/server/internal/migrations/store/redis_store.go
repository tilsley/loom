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
	redisIndexKey        = "migrations:index"
	redisKeyPrefix       = "migration:"
	redisCandidatePrefix = "candidate:"        // candidate:{migrationId}:{candidateId}
	redisCandidateIndex  = "candidates:index:" // candidates:index:{migrationId}
)

func candidateKey(migrationID, candidateID string) string {
	return redisCandidatePrefix + migrationID + ":" + candidateID
}

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

// getCandidates fetches all candidates for a migration from their individual keys.
func (s *RedisMigrationStore) getCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error) {
	ids, err := s.rdb.SMembers(ctx, redisCandidateIndex+migrationID).Result()
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = candidateKey(migrationID, id)
	}
	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget candidates: %w", err)
	}
	candidates := make([]api.Candidate, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		var c api.Candidate
		if err := json.Unmarshal([]byte(v.(string)), &c); err != nil {
			return nil, fmt.Errorf("unmarshal candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// Save persists a migration and each candidate in its own key.
// The migration JSON is stored with Candidates nil to avoid duplication.
func (s *RedisMigrationStore) Save(ctx context.Context, m api.Migration) error {
	for _, c := range m.Candidates {
		data, err := json.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshal candidate %q: %w", c.Id, err)
		}
		if err := s.rdb.Set(ctx, candidateKey(m.Id, c.Id), data, 0).Err(); err != nil {
			return fmt.Errorf("save candidate %q: %w", c.Id, err)
		}
		if err := s.rdb.SAdd(ctx, redisCandidateIndex+m.Id, c.Id).Err(); err != nil {
			return fmt.Errorf("update candidate index for %q: %w", c.Id, err)
		}
	}
	m.Candidates = nil
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
// Candidates are populated from their individual keys.
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
	candidates, err := s.getCandidates(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Candidates = candidates
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

// SetCandidateStatus updates the status of a candidate via its individual key.
func (s *RedisMigrationStore) SetCandidateStatus(
	ctx context.Context,
	migrationID, candidateID string,
	status api.CandidateStatus,
) error {
	key := candidateKey(migrationID, candidateID)
	val, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return fmt.Errorf("candidate %q not found in migration %q", candidateID, migrationID)
	}
	if err != nil {
		return fmt.Errorf("get candidate %q: %w", candidateID, err)
	}
	var c api.Candidate
	if err := json.Unmarshal([]byte(val), &c); err != nil {
		return fmt.Errorf("unmarshal candidate %q: %w", candidateID, err)
	}
	c.Status = &status
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal candidate %q: %w", candidateID, err)
	}
	return s.rdb.Set(ctx, key, data, 0).Err()
}

// SaveCandidates merges the incoming candidate list into individual candidate keys.
// Candidates already in running or completed state are preserved as-is.
func (s *RedisMigrationStore) SaveCandidates(ctx context.Context, migrationID string, incoming []api.Candidate) error {
	exists, err := s.rdb.Exists(ctx, redisKeyPrefix+migrationID).Result()
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationID, err)
	}
	if exists == 0 {
		return fmt.Errorf("migration %q not found", migrationID)
	}

	existing := map[string]api.Candidate{}
	existingIDs, err := s.rdb.SMembers(ctx, redisCandidateIndex+migrationID).Result()
	if err != nil {
		return fmt.Errorf("get candidate index: %w", err)
	}
	if len(existingIDs) > 0 {
		keys := make([]string, len(existingIDs))
		for i, id := range existingIDs {
			keys[i] = candidateKey(migrationID, id)
		}
		vals, err := s.rdb.MGet(ctx, keys...).Result()
		if err != nil {
			return fmt.Errorf("mget existing candidates: %w", err)
		}
		for _, v := range vals {
			if v == nil {
				continue
			}
			var c api.Candidate
			if err := json.Unmarshal([]byte(v.(string)), &c); err != nil {
				return fmt.Errorf("unmarshal candidate: %w", err)
			}
			existing[c.Id] = c
		}
	}

	notStarted := api.CandidateStatusNotStarted
	incomingIDs := make(map[string]bool, len(incoming))
	for _, c := range incoming {
		incomingIDs[c.Id] = true
		if ex, ok := existing[c.Id]; ok && ex.Status != nil &&
			(*ex.Status == api.CandidateStatusRunning || *ex.Status == api.CandidateStatusCompleted) {
			// preserve running/completed candidate as-is
			continue
		}
		c.Status = &notStarted
		data, err := json.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshal candidate %q: %w", c.Id, err)
		}
		if err := s.rdb.Set(ctx, candidateKey(migrationID, c.Id), data, 0).Err(); err != nil {
			return fmt.Errorf("save candidate %q: %w", c.Id, err)
		}
		if err := s.rdb.SAdd(ctx, redisCandidateIndex+migrationID, c.Id).Err(); err != nil {
			return fmt.Errorf("update candidate index %q: %w", c.Id, err)
		}
	}

	// Keep running/completed candidates not in incoming list in the index.
	for _, ex := range existing {
		if !incomingIDs[ex.Id] && ex.Status != nil &&
			(*ex.Status == api.CandidateStatusRunning || *ex.Status == api.CandidateStatusCompleted) {
			s.rdb.SAdd(ctx, redisCandidateIndex+migrationID, ex.Id) //nolint:errcheck
		}
	}

	return nil
}

// UpdateCandidateMetadata merges the given key-value pairs into a candidate's metadata.
func (s *RedisMigrationStore) UpdateCandidateMetadata(
	ctx context.Context,
	migrationID, candidateID string,
	metadata map[string]string,
) error {
	key := candidateKey(migrationID, candidateID)
	val, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return fmt.Errorf("candidate %q not found in migration %q", candidateID, migrationID)
	}
	if err != nil {
		return fmt.Errorf("get candidate %q: %w", candidateID, err)
	}
	var c api.Candidate
	if err := json.Unmarshal([]byte(val), &c); err != nil {
		return fmt.Errorf("unmarshal candidate %q: %w", candidateID, err)
	}
	if c.Metadata == nil {
		c.Metadata = &map[string]string{}
	}
	for k, v := range metadata {
		(*c.Metadata)[k] = v
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal candidate %q: %w", candidateID, err)
	}
	return s.rdb.Set(ctx, key, data, 0).Err()
}

// GetCandidates returns the candidate list for a migration with their current status.
func (s *RedisMigrationStore) GetCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error) {
	candidates, err := s.getCandidates(ctx, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get candidates for %q: %w", migrationID, err)
	}
	return candidates, nil
}

