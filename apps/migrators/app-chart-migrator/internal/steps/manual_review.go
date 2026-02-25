package steps

import (
	"context"

	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
	"github.com/tilsley/loom/pkg/api"
)

// ManualReview is a no-op step handler. The worker acknowledges dispatch and
// does nothing — the workflow waits for an operator to approve via the UI,
// which sends a step-completed event directly to the Loom server.
type ManualReview struct{}

// Execute returns a result with no PR, signalling to the dispatch handler that
// this is a manual step. Instructions are read from the step config so the UI
// can display what the operator needs to do before marking it done.
func (h *ManualReview) Execute(_ context.Context, _ gitrepo.Client, _ *Config, req api.DispatchStepRequest) (*Result, error) {
	var instructions string
	if req.Config != nil {
		instructions = (*req.Config)["instructions"]
	}
	return &Result{Instructions: instructions}, nil
}
