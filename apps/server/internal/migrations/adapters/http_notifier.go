package adapters

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
var _ migrations.WorkerNotifier = (*HTTPWorkerNotifier)(nil)

// HTTPWorkerNotifier implements WorkerNotifier by posting DispatchStepRequest
// directly to the worker's /dispatch-step endpoint over HTTP.
type HTTPWorkerNotifier struct {
	client *http.Client
}

// NewHTTPWorkerNotifier creates a new HTTPWorkerNotifier.
func NewHTTPWorkerNotifier(client *http.Client) *HTTPWorkerNotifier {
	return &HTTPWorkerNotifier{client: client}
}

// Dispatch sends a step request directly to the worker URL carried in req.WorkerUrl.
func (n *HTTPWorkerNotifier) Dispatch(ctx context.Context, req api.DispatchStepRequest) error {
	if req.WorkerUrl == "" {
		return fmt.Errorf("no worker URL in dispatch request for step %q", req.StepName)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal dispatch request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.WorkerUrl+"/dispatch-step", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create dispatch request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("notify worker %q: %w", req.WorkerApp, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		return fmt.Errorf("worker %q returned HTTP %d", req.WorkerApp, resp.StatusCode)
	}
	return nil
}
