package pending

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "pending-callback:"

// Callback holds the Loom callback info for a PR awaiting merge.
type Callback struct {
	CallbackID  string `json:"callbackId"`
	StepName    string `json:"stepName"`
	CandidateId string `json:"candidateId"`
	PRURL       string `json:"prUrl"`
}

// Store persists pending PR-to-workflow callback mappings in Redis
// so they survive worker restarts.
type Store struct {
	rdb *redis.Client
	log *slog.Logger
}

// NewStore creates a new pending callback Store connected to the given Redis address.
func NewStore(redisAddr string, log *slog.Logger) *Store {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	return &Store{rdb: rdb, log: log}
}

// Add persists a pending callback keyed by "owner/repo#number".
func (s *Store) Add(key string, cb Callback) {
	data, err := json.Marshal(cb)
	if err != nil {
		s.log.Error("failed to marshal pending callback", "key", key, "error", err)
		return
	}

	stateKey := keyPrefix + key
	if err := s.rdb.Set(context.Background(), stateKey, data, 0).Err(); err != nil {
		s.log.Error("failed to save pending callback to Redis", "key", key, "error", err)
	}
}

// Remove retrieves and atomically deletes a pending callback by key.
func (s *Store) Remove(key string) (Callback, bool) {
	stateKey := keyPrefix + key

	val, err := s.rdb.GetDel(context.Background(), stateKey).Result()
	if errors.Is(err, redis.Nil) {
		return Callback{}, false
	}
	if err != nil {
		s.log.Error("failed to get pending callback from Redis", "key", key, "error", err)
		return Callback{}, false
	}

	var cb Callback
	if err := json.Unmarshal([]byte(val), &cb); err != nil {
		s.log.Error("failed to unmarshal pending callback", "key", key, "error", err)
		return Callback{}, false
	}
	return cb, true
}
