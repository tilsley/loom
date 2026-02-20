package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/github"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// GenerateAppChart creates a Helm chart directory in the app repo with
// Chart.yaml, base values, per-env values, and an OCI publish workflow.
type GenerateAppChart struct{}

// Execute implements Handler.
func (h *GenerateAppChart) Execute(
	ctx context.Context,
	gh *github.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Target)
	owner, repo := targetOwnerRepo(req.Target)

	files := make(map[string]string)

	// Chart.yaml
	files[fmt.Sprintf("charts/%s/Chart.yaml", app)] = fmt.Sprintf(`apiVersion: v2
name: %s
description: Helm chart for %s
type: application
version: 0.1.0
appVersion: "1.0.0"
`, app, app)

	// Base values.yaml
	files[fmt.Sprintf("charts/%s/values.yaml", app)] = fmt.Sprintf(`nameOverride: %s
image:
  repository: acme/%s
  tag: latest
replicaCount: 1
serviceMonitor:
  enabled: true
`, app, app)

	// Per-env values extracted from gitops overlay helm.parameters
	for _, env := range cfg.Envs {
		path := fmt.Sprintf("apps/%s/overlays/%s/application.yaml", app, env)
		fc, err := gh.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, path)
		if err != nil {
			return nil, fmt.Errorf("get %s: %w", path, err)
		}

		envValues, err := extractEnvValues(fc.Content)
		if err != nil {
			return nil, fmt.Errorf("extract values from %s: %w", path, err)
		}

		files[fmt.Sprintf("charts/%s/values-%s.yaml", app, env)] = envValues
	}

	// OCI publish workflow
	files[".github/workflows/publish-chart.yaml"] = fmt.Sprintf(`name: Publish Chart
on:
  push:
    branches: [main]
    paths:
      - 'charts/**'
jobs:
  publish:
    runs-on: ubuntu-latest
    permissions:
      packages: write
    steps:
      - uses: actions/checkout@v4
      - name: Login to GHCR
        run: echo "${{ secrets.GITHUB_TOKEN }}" | helm registry login ghcr.io -u ${{ github.actor }} --password-stdin
      - name: Package chart
        run: helm package charts/%s
      - name: Push chart
        run: helm push %s-0.1.0.tgz oci://ghcr.io/acme
`, app, app)

	return &Result{
		Owner: owner,
		Repo:  repo,
		Title: fmt.Sprintf("[%s] Generate app chart for %s", req.MigrationId, app),
		Body: fmt.Sprintf(
			"Create app-specific Helm chart `charts/%s/` with per-env values and OCI publish workflow.",
			app,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  files,
	}, nil
}

// extractEnvValues parses an Argo Application YAML and converts helm.parameters
// into a flat values YAML file.
func extractEnvValues(content string) (string, error) {
	data, err := yamlutil.Parse(content)
	if err != nil {
		return "", err
	}

	helm, err := yamlutil.GetMap(data, "spec", "source", "helm")
	if err != nil {
		return "", fmt.Errorf("get spec.source.helm: %w", err)
	}

	params, ok := helm["parameters"].([]interface{})
	if !ok {
		return "# no parameters\n", nil
	}

	values := make(map[string]interface{})
	for _, p := range params {
		param, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := param["name"].(string)   //nolint:errcheck // zero value is correct for absent/non-string keys
		value, _ := param["value"].(string) //nolint:errcheck // zero value is correct for absent/non-string keys
		if name != "" {
			values[name] = value
		}
	}

	return yamlutil.Marshal(values)
}
