package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// DisableResourcePrune adds Prune=false sync option to ServiceMonitor resources
// so ArgoCD won't delete them during the chart swap.
//
// Service monitors are expected to sit in the same directory as the discovered
// Application manifest for each environment:
//
//	src/<team>/<system>/<env>/<cloud>/<namespace>/service-monitor.yaml
//
// All found service monitors are patched in a single PR.
type DisableResourcePrune struct{}

// Execute implements Handler.
func (h *DisableResourcePrune) Execute(
	ctx context.Context,
	gr gitrepo.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)

	prFiles := make(map[string]string)
	if req.Candidate.Files != nil {
		for _, group := range *req.Candidate.Files {
			if group.Name == "app-repo" || len(group.Files) == 0 {
				continue
			}
			// Derive the service-monitor path from the Application file's directory.
			appPath := group.Files[0].Path
			dir := appPath[:strings.LastIndex(appPath, "/")]
			smPath := dir + "/service-monitor.yaml"

			fc, err := gr.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, smPath)
			if err != nil {
				// No service monitor in this env directory — skip silently.
				continue
			}

			root, err := yamlutil.ParseNode(fc.Content)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", smPath, err)
			}

			annotations := yamlutil.EnsureMappingNode(root, "metadata", "annotations")
			yamlutil.SetScalar(annotations, "argocd.argoproj.io/sync-options", "Prune=false")

			out, err := yamlutil.MarshalNode(root)
			if err != nil {
				return nil, err
			}
			prFiles[smPath] = out
		}
	}

	return &Result{
		Owner: cfg.GitopsOwner,
		Repo:  cfg.GitopsRepo,
		Title: fmt.Sprintf("[%s] Disable resource pruning for %s", req.MigrationId, app),
		Body: fmt.Sprintf(
			"Add `Prune=false` sync option to ServiceMonitor resources for `%s`.\n\nThis prevents ArgoCD from deleting them during the chart swap.",
			app,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  prFiles,
	}, nil
}
