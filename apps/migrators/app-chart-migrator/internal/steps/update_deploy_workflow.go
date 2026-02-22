package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
	"github.com/tilsley/loom/pkg/api"
)

// UpdateDeployWorkflow updates the app repo's CI workflow to include
// the chart publish step for the new OCI app chart.
type UpdateDeployWorkflow struct{}

// Execute implements Handler.
func (h *UpdateDeployWorkflow) Execute(
	_ context.Context,
	_ gitrepo.Client,
	_ *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)
	owner, repo := candidateOwnerRepo(req.Candidate)

	updatedCI := fmt.Sprintf(`name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build
        run: make build
      - name: Test
        run: make test

  publish-chart:
    needs: build
    if: github.ref == 'refs/heads/main'
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
        run: helm push %s-*.tgz oci://ghcr.io/acme
`, app, app)

	return &Result{
		Owner: owner,
		Repo:  repo,
		Title: fmt.Sprintf("[%s] Update CI workflow for %s chart deployment", req.MigrationId, app),
		Body: fmt.Sprintf(
			"Add `publish-chart` job to CI workflow for `%s`. Charts are published to `oci://ghcr.io/acme` on merge to main.",
			app,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files: map[string]string{
			".github/workflows/ci.yaml": updatedCI,
		},
	}, nil
}
