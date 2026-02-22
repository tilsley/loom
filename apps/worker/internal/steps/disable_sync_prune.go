package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// DisableSyncPrune sets syncPolicy.automated.prune to false on the Argo
// Application for a specific environment, preventing auto-deletion during
// the chart swap.
type DisableSyncPrune struct{}

// Execute implements Handler.
func (h *DisableSyncPrune) Execute(
	ctx context.Context,
	gr gitrepo.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)
	env := (*req.Config)["env"]
	path := fmt.Sprintf("apps/%s/overlays/%s/application.yaml", app, env)

	fc, err := gr.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, path)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}

	root, err := yamlutil.ParseNode(fc.Content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	yamlutil.SetNestedValue(root, false, "spec", "syncPolicy", "automated", "prune")

	out, err := yamlutil.MarshalNode(root)
	if err != nil {
		return nil, err
	}

	return &Result{
		Owner:  cfg.GitopsOwner,
		Repo:   cfg.GitopsRepo,
		Title:  fmt.Sprintf("[%s] Disable sync pruning for %s (%s)", req.MigrationId, app, env),
		Body:   fmt.Sprintf("Set `syncPolicy.automated.prune: false` on the `%s` Argo Application in `%s`.", app, env),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  map[string]string{path: out},
	}, nil
}
