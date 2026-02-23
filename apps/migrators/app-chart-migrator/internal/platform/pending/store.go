package pending

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	dapr "github.com/dapr/go-sdk/client"
)

const stateStoreName = "statestore"
const keyPrefix = "pending-callback:"

// Callback holds the Loom callback info for a PR awaiting merge.
type Callback struct {
	CallbackID  string `json:"callbackId"`
	StepName    string `json:"stepName"`
	CandidateId string `json:"candidateId"`
	PRURL       string `json:"prUrl"`
}

// Store persists pending PR-to-workflow callback mappings in Dapr state store
// so they survive worker restarts.
type Store struct {
	client dapr.Client
	log    *slog.Logger
}

// NewStore creates a new pending callback Store.
func NewStore(client dapr.Client, log *slog.Logger) *Store {
	return &Store{client: client, log: log}
}

// Add persists a pending callback keyed by "owner/repo#number".
func (s *Store) Add(key string, cb Callback) {
	data, err := json.Marshal(cb)
	if err != nil {
		s.log.Error("failed to marshal pending callback", "key", key, "error", err)
		return
	}

	ctx := context.Background()
	stateKey := keyPrefix + key
	if err := s.client.SaveState(ctx, stateStoreName, stateKey, data, nil); err != nil {
		s.log.Error("failed to save pending callback to state store", "key", key, "error", err)
	}
}

// Remove retrieves and deletes a pending callback by key.
func (s *Store) Remove(key string) (Callback, bool) {
	ctx := context.Background()
	stateKey := keyPrefix + key

	item, err := s.client.GetState(ctx, stateStoreName, stateKey, nil)
	if err != nil {
		s.log.Error("failed to get pending callback from state store", "key", key, "error", err)
		return Callback{}, false
	}

	if item.Value == nil {
		return Callback{}, false
	}

	var cb Callback
	if err := json.Unmarshal(item.Value, &cb); err != nil {
		s.log.Error("failed to unmarshal pending callback", "key", key, "error", err)
		return Callback{}, false
	}

	// Delete the key after successful read
	if err := s.client.DeleteState(ctx, stateStoreName, stateKey, nil); err != nil {
		s.log.Warn(
			fmt.Sprintf("failed to delete pending callback key %q (callback will still be processed)", key),
			"error",
			err,
		)
	}

	return cb, true
}
