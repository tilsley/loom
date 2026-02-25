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

// Compile-time check: *HTTPWorkerNotifier implements migrations.WorkerNotifier.
var _ migrations.MigratorNotifier = (*HTTPMigratorNotifier)(nil)

// HTTPMigratorNotifier implements MigratorNotifier by posting DispatchStepRequest
// directly to the worker's /dispatch-step endpoint over HTTP.
type HTTPMigratorNotifier struct {
	client *http.Client
}

// NewHTTPMigratorNotifier creates a new HTTPMigratorNotifier.
func NewHTTPMigratorNotifier(client *http.Client) *HTTPMigratorNotifier {
	return &HTTPMigratorNotifier{client: client}
}

// Dispatch sends a step request directly to the migrator URL carried in req.MigratorUrl.
func (n *HTTPMigratorNotifier) Dispatch(ctx context.Context, req api.DispatchStepRequest) error {
	if req.MigratorUrl == "" {
		return fmt.Errorf("no worker URL in dispatch request for step %q", req.StepName)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal dispatch request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.MigratorUrl+"/dispatch-step", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create dispatch request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("notify migrator %q: %w", req.MigratorApp, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		return fmt.Errorf("migrator %q returned HTTP %d", req.MigratorApp, resp.StatusCode)
	}
	return nil
}
