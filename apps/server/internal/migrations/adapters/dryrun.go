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
// via Dapr service invocation.
type DaprDryRunAdapter struct {
	client    dapr.Client
	workerApp string
}

// NewDaprDryRunAdapter creates a DaprDryRunAdapter that invokes the given Dapr app-id.
func NewDaprDryRunAdapter(client dapr.Client, workerApp string) *DaprDryRunAdapter {
	return &DaprDryRunAdapter{client: client, workerApp: workerApp}
}

// DryRun serialises the request and sends it to the worker via Dapr service invocation.
func (a *DaprDryRunAdapter) DryRun(ctx context.Context, req api.DryRunRequest) (*api.DryRunResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal dry-run request: %w", err)
	}

	out, err := a.client.InvokeMethodWithContent(ctx, a.workerApp, "dry-run", "post", &dapr.DataContent{
		ContentType: "application/json",
		Data:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke dry-run on %s: %w", a.workerApp, err)
	}

	var result api.DryRunResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("unmarshal dry-run result: %w", err)
	}
	return &result, nil
}
