package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// SwapChart changes the Argo Application source from the generic Helm chart
// to the app-specific OCI chart, and removes helm.parameters (now in the
// app chart's per-env values files).
type SwapChart struct{}

// Execute implements Handler.
func (h *SwapChart) Execute(
	ctx context.Context,
	gr gitrepo.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)
	env := (*req.Config)["env"]
	path, ok := gitopsFileForEnv(req.Candidate, env)
	if !ok {
		return nil, fmt.Errorf("no gitops file found for env %q in candidate %q", env, app)
	}

	fc, err := gr.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, path)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}

	root, err := yamlutil.ParseNode(fc.Content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	source, err := yamlutil.GetMappingNode(root, "spec", "source")
	if err != nil {
		return nil, fmt.Errorf("get spec.source: %w", err)
	}

	// Swap to OCI app chart
	yamlutil.SetScalar(source, "repoURL", fmt.Sprintf("oci://ghcr.io/acme/%s-chart", app))
	yamlutil.SetScalar(source, "targetRevision", "0.1.0")
	yamlutil.DeleteKey(source, "chart")

	// Remove helm.parameters (migrated to app chart values files)
	helm, err := yamlutil.GetMappingNode(root, "spec", "source", "helm")
	if err == nil {
		yamlutil.DeleteKey(helm, "parameters")
	}

	out, err := yamlutil.MarshalNode(root)
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
