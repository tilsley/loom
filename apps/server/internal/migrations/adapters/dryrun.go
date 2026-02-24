package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	dapr "github.com/dapr/go-sdk/client"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// Compile-time check: *DaprDryRunAdapter implements migrations.DryRunner.
var _ migrations.DryRunner = (*DaprDryRunAdapter)(nil)

// DaprDryRunAdapter implements DryRunner by calling a worker's /dry-run endpoint
// via Dapr service invocation. The target worker is derived from the request's
// step definitions at call time — the server has no hardcoded knowledge of migrators.
type DaprDryRunAdapter struct {
	client dapr.Client
}

// NewDaprDryRunAdapter creates a DaprDryRunAdapter.
func NewDaprDryRunAdapter(client dapr.Client) *DaprDryRunAdapter {
	return &DaprDryRunAdapter{client: client}
}

// DryRun serialises the request and sends it to the worker via Dapr service invocation.
// The target is the first step whose workerApp is not "loom" (loom owns manual-review steps).
func (a *DaprDryRunAdapter) DryRun(ctx context.Context, req api.DryRunRequest) (*api.DryRunResult, error) {
	workerApp := ""
	for _, s := range req.Steps {
		if s.WorkerApp != "loom" {
			workerApp = s.WorkerApp
			break
		}
	}
	if workerApp == "" {
		return nil, fmt.Errorf("no external worker found in migration steps")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal dry-run request: %w", err)
	}

	out, err := a.client.InvokeMethodWithContent(ctx, workerApp, "dry-run", "post", &dapr.DataContent{
		ContentType: "application/json",
		Data:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke dry-run on %s: %w", workerApp, err)
	}

	var result api.DryRunResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("unmarshal dry-run result: %w", err)
	}
	return &result, nil
}
