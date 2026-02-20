package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/github"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// EnableSyncPrune re-enables syncPolicy.automated.prune on the Argo
// Application for a specific environment after the chart swap is complete.
type EnableSyncPrune struct{}

// Execute implements Handler.
func (h *EnableSyncPrune) Execute(
	ctx context.Context,
	gh *github.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Target)
	env := (*req.Config)["env"]
	path := fmt.Sprintf("apps/%s/overlays/%s/application.yaml", app, env)

	fc, err := gh.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, path)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}

	data, err := yamlutil.Parse(fc.Content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	yamlutil.SetNested(data, true, "spec", "syncPolicy", "automated", "prune")

	out, err := yamlutil.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Result{
		Owner: cfg.GitopsOwner,
		Repo:  cfg.GitopsRepo,
		Title: fmt.Sprintf("[%s] Re-enable sync pruning for %s (%s)", req.MigrationId, app, env),
		Body: fmt.Sprintf(
			"Set `syncPolicy.automated.prune: true` on `%s` in `%s` now that the chart swap is complete.",
			app,
			env,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  map[string]string{path: out},
	}, nil
}
