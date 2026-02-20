package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/github"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// DisableResourcePrune adds Prune=false sync option to non-Argo-app resources
// (e.g. ServiceMonitor) so ArgoCD won't delete them during the chart swap.
type DisableResourcePrune struct{}

// Execute implements Handler.
func (h *DisableResourcePrune) Execute(
	ctx context.Context,
	gh *github.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Target)
	path := fmt.Sprintf("apps/%s/base/service-monitor.yaml", app)

	fc, err := gh.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, path)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}

	data, err := yamlutil.Parse(fc.Content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	// Ensure metadata.annotations exists
	metadata, err := yamlutil.GetMap(data, "metadata")
	if err != nil {
		return nil, fmt.Errorf("get metadata: %w", err)
	}
	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		annotations = make(map[string]interface{})
		metadata["annotations"] = annotations
	}
	annotations["argocd.argoproj.io/sync-options"] = "Prune=false"

	out, err := yamlutil.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Result{
		Owner: cfg.GitopsOwner,
		Repo:  cfg.GitopsRepo,
		Title: fmt.Sprintf("[%s] Disable resource pruning for %s", req.MigrationId, app),
		Body: fmt.Sprintf(
			"Add `Prune=false` sync option to non-Argo resources for `%s`.\n\nThis prevents ArgoCD from deleting these resources during the chart swap.",
			app,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  map[string]string{path: out},
	}, nil
}
