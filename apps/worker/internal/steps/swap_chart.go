package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/github"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// SwapChart changes the Argo Application source from the generic Helm chart
// to the app-specific OCI chart, and removes helm.parameters (now in the
// app chart's per-env values files).
type SwapChart struct{}

// Execute implements Handler.
func (h *SwapChart) Execute(
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

	source, err := yamlutil.GetMap(data, "spec", "source")
	if err != nil {
		return nil, fmt.Errorf("get spec.source: %w", err)
	}

	// Swap to OCI app chart
	source["repoURL"] = fmt.Sprintf("oci://ghcr.io/acme/%s-chart", app)
	source["targetRevision"] = "0.1.0"
	delete(source, "chart")

	// Remove helm.parameters (migrated to app chart values files)
	helm, err := yamlutil.GetMap(data, "spec", "source", "helm")
	if err == nil {
		delete(helm, "parameters")
	}

	out, err := yamlutil.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Result{
		Owner: cfg.GitopsOwner,
		Repo:  cfg.GitopsRepo,
		Title: fmt.Sprintf("[%s] Swap to OCI app chart for %s (%s)", req.MigrationId, app, env),
		Body: fmt.Sprintf(
			"Switch `%s` in `%s` from generic chart to `oci://ghcr.io/acme/%s-chart` and remove `helm.parameters`.",
			app,
			env,
			app,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  map[string]string{path: out},
	}, nil
}
