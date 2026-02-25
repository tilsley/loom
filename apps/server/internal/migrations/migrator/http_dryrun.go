package migrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// Compile-time check: *HTTPDryRunAdapter implements migrations.DryRunner.
var _ migrations.DryRunner = (*HTTPDryRunAdapter)(nil)

// HTTPDryRunAdapter implements DryRunner by posting a DryRunRequest directly
// to the worker's /dry-run endpoint over HTTP.
type HTTPDryRunAdapter struct {
	client *http.Client
}

// NewHTTPDryRunAdapter creates a new HTTPDryRunAdapter.
func NewHTTPDryRunAdapter(client *http.Client) *HTTPDryRunAdapter {
	return &HTTPDryRunAdapter{client: client}
}

// DryRun POSTs the request to {migratorUrl}/dry-run and returns the result.
func (d *HTTPDryRunAdapter) DryRun(ctx context.Context, migratorUrl string, req api.DryRunRequest) (*api.DryRunResult, error) {
	if migratorUrl == "" {
		return nil, fmt.Errorf("no migrator URL configured for migration %q", req.MigrationId)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal dry-run request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, migratorUrl+"/dry-run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create dry-run request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("invoke dry-run: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dry-run returned HTTP %d", resp.StatusCode)
	}

	var result api.DryRunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode dry-run result: %w", err)
	}
	return &result, nil
}
