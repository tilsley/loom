package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// GenerateAppChart creates a Helm chart directory in the app repo with
// Chart.yaml, base values, per-env values, and an OCI publish workflow.
type GenerateAppChart struct{}

// Execute implements Handler.
func (h *GenerateAppChart) Execute(
	ctx context.Context,
	gr gitrepo.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)
	owner, repo := candidateOwnerRepo(req.Candidate)

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

	// Per-env values extracted from the candidate's discovered gitops Application files.
	// Each env FileGroup (e.g. "dev", "staging", "prod") holds the real file path.
	if req.Candidate.Files != nil {
		for _, group := range *req.Candidate.Files {
			if group.Name == "app-repo" || group.Name == "base" || len(group.Files) == 0 {
				continue
			}
			envPath := group.Files[0].Path
			fc, err := gr.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, envPath)
			if err != nil {
				return nil, fmt.Errorf("get %s: %w", envPath, err)
			}
			envValues, err := extractEnvValues(fc.Content)
			if err != nil {
				return nil, fmt.Errorf("extract values from %s: %w", envPath, err)
			}
			files[fmt.Sprintf("charts/%s/values-%s.yaml", app, group.Name)] = envValues
		}
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
// into a nested values YAML file. Dotted parameter names (e.g. "image.tag") are
// expanded into nested maps so the output is valid Helm values syntax.
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
			setNestedValue(values, strings.Split(name, "."), value)
		}
	}

	return yamlutil.Marshal(values)
}

// setNestedValue sets value at a dotted-path within a nested map[string]interface{}.
// e.g. setNestedValue(m, ["image","tag"], "latest") produces m["image"]["tag"] = "latest".
func setNestedValue(m map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}
	if _, ok := m[keys[0]]; !ok {
		m[keys[0]] = make(map[string]interface{})
	}
	if sub, ok := m[keys[0]].(map[string]interface{}); ok {
		setNestedValue(sub, keys[1:], value)
	}
}
