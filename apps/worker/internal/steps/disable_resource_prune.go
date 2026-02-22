package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// DisableResourcePrune adds Prune=false sync option to non-Argo-app resources
// (e.g. ServiceMonitor) so ArgoCD won't delete them during the chart swap.
type DisableResourcePrune struct{}

// Execute implements Handler.
func (h *DisableResourcePrune) Execute(
	ctx context.Context,
	gr gitrepo.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)
	path := fmt.Sprintf("apps/%s/base/service-monitor.yaml", app)

	fc, err := gr.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, path)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}

	root, err := yamlutil.ParseNode(fc.Content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	annotations := yamlutil.EnsureMappingNode(root, "metadata", "annotations")
	yamlutil.SetScalar(annotations, "argocd.argoproj.io/sync-options", "Prune=false")

	out, err := yamlutil.MarshalNode(root)
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
