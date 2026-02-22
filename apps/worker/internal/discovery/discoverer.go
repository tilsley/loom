package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tilsley/loom/pkg/api"
)

// Discoverer scans a source of truth (e.g. a GitOps repo) and returns
// the list of candidates that need migration.
type Discoverer interface {
	Discover(ctx context.Context) ([]api.Candidate, error)
}

// Runner runs discovery once and POSTs the results to the server.
// Call as a goroutine after the worker HTTP server is up.
type Runner struct {
	MigrationID string
	Discoverer  Discoverer
	ServerURL   string
	Log         *slog.Logger
}

// Run executes discovery once, then submits the candidates to the server.
// It retries the submission if the migration isn't registered yet (the pub/sub
// announcement may still be in flight when discovery finishes).
func (r *Runner) Run(ctx context.Context) {
	candidates, err := r.Discoverer.Discover(ctx)
	if err != nil {
		r.Log.Error("discovery failed", "migrationID", r.MigrationID, "error", err)
		return
	}

	r.Log.Info("discovered candidates", "migrationID", r.MigrationID, "count", len(candidates))

	body, err := json.Marshal(api.SubmitCandidatesRequest{Candidates: candidates})
	if err != nil {
		r.Log.Error("marshal candidates failed", "error", err)
		return
	}

	url := fmt.Sprintf("%s/migrations/%s/candidates", r.ServerURL, r.MigrationID)

	for attempt := range 15 {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}

		// Re-create the reader each attempt — a consumed reader sends an empty body.
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			r.Log.Error("build request failed", "error", err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			r.Log.Warn("submit candidates failed, retrying", "attempt", attempt+1, "error", err)
			continue
		}
		_ = resp.Body.Close()

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			r.Log.Info("candidates submitted", "migrationID", r.MigrationID, "count", len(candidates))
			return
		case resp.StatusCode == 404:
			r.Log.Warn("migration not registered yet, retrying", "migrationID", r.MigrationID, "attempt", attempt+1)
		default:
			r.Log.Error("submit candidates returned non-2xx", "migrationID", r.MigrationID, "status", resp.StatusCode)
			return
		}
	}

	r.Log.Error("failed to submit candidates after retries", "migrationID", r.MigrationID)
}
