package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// CleanupCommon removes the old helm.values from the base application YAML
// now that each environment uses the app chart's own values files.
type CleanupCommon struct{}

// Execute implements Handler.
func (h *CleanupCommon) Execute(
	ctx context.Context,
	gr gitrepo.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)
	path := fmt.Sprintf("apps/%s/base/application.yaml", app)

	fc, err := gr.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, path)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}

	root, err := yamlutil.ParseNode(fc.Content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	helm, err := yamlutil.GetMappingNode(root, "spec", "source", "helm")
	if err == nil {
		yamlutil.DeleteKey(helm, "values")
	}

	out, err := yamlutil.MarshalNode(root)
	if err != nil {
		return nil, err
	}

	return &Result{
		Owner: cfg.GitopsOwner,
		Repo:  cfg.GitopsRepo,
		Title: fmt.Sprintf("[%s] Clean up common helm values for %s", req.MigrationId, app),
		Body: fmt.Sprintf(
			"Remove old `helm.values` from the base application for `%s`. Values are now in the app chart.",
			app,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  map[string]string{path: out},
	}, nil
}
