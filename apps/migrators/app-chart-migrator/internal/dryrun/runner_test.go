package dryrun_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	githubadapter "github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/adapters/github"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/dryrun"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/steps"
	"github.com/tilsley/loom/pkg/api"
)

const gitopsOwner = "tilsley"
const gitopsRepo = "gitops"

func stepCfg() *steps.Config {
	return &steps.Config{
		GitopsOwner: gitopsOwner,
		GitopsRepo:  gitopsRepo,
		Envs:        []string{"dev"},
	}
}

func strPtr(s string) *string { return &s }

// TestRunner_SkipsNilType verifies that steps without a Type are skipped.
func TestRunner_SkipsNilType(t *testing.T) {
	r := &dryrun.Runner{RealClient: githubadapter.NewInMem(), StepCfg: stepCfg()}

	req := api.DryRunRequest{
		MigrationId: "mig",
		Candidate:   api.Candidate{Id: "app"},
		Steps: []api.StepDefinition{
			{Name: "no-type", MigratorApp: "app-chart-migrator"},
		},
	}

	result, err := r.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.True(t, result.Steps[0].Skipped)
}

// TestRunner_SkipsUnknownType verifies that steps with an unregistered Type are skipped.
func TestRunner_SkipsUnknownType(t *testing.T) {
	r := &dryrun.Runner{RealClient: githubadapter.NewInMem(), StepCfg: stepCfg()}

	req := api.DryRunRequest{
		MigrationId: "mig",
		Candidate:   api.Candidate{Id: "app"},
		Steps: []api.StepDefinition{
			{Name: "unknown", MigratorApp: "app-chart-migrator", Type: strPtr("not-a-real-step")},
		},
	}

	result, err := r.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.True(t, result.Steps[0].Skipped)
}

// TestRunner_SkipsOtherWorkerApp verifies that steps for a different worker are skipped.
func TestRunner_SkipsOtherWorkerApp(t *testing.T) {
	r := &dryrun.Runner{RealClient: githubadapter.NewInMem(), StepCfg: stepCfg()}

	req := api.DryRunRequest{
		MigrationId: "mig",
		Candidate:   api.Candidate{Id: "app"},
		Steps: []api.StepDefinition{
			{Name: "other-worker-step", MigratorApp: "some-other-worker", Type: strPtr("disable-base-resource-prune")},
		},
	}

	result, err := r.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.True(t, result.Steps[0].Skipped)
}

// TestRunner_ManualReview_NoFiles verifies that manual-review steps execute
// but produce no file diffs (the workflow blocks waiting for a UI approval instead).
func TestRunner_ManualReview_NoFiles(t *testing.T) {
	r := &dryrun.Runner{RealClient: githubadapter.NewInMem(), StepCfg: stepCfg()}

	req := api.DryRunRequest{
		MigrationId: "mig",
		Candidate:   api.Candidate{Id: "app"},
		Steps: []api.StepDefinition{
			{
				Name:      "review-dev",
				MigratorApp: "app-chart-migrator",
				Type:      strPtr("manual-review"),
				Config:    &map[string]string{"instructions": "check ArgoCD"},
			},
		},
	}

	result, err := r.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.False(t, result.Steps[0].Skipped)
	assert.Empty(t, *result.Steps[0].Files)
}

// TestRunner_ExecutesStep_ProducesDiff verifies that a registered step type
// is executed and returns file diffs when the step modifies files.
func TestRunner_ExecutesStep_ProducesDiff(t *testing.T) {
	gh := githubadapter.NewInMem()
	gh.SetFile(gitopsOwner, gitopsRepo, "apps/my-app/base.yaml",
		"apiVersion: argoproj.io/v1alpha1\nkind: Deployment\nmetadata:\n  name: my-app\n")

	r := &dryrun.Runner{RealClient: gh, StepCfg: stepCfg()}

	candidate := api.Candidate{
		Id: "my-app",
		Files: &[]api.FileGroup{
			{
				Name:  "base",
				Files: []api.FileRef{{Path: "apps/my-app/base.yaml"}},
			},
		},
	}

	req := api.DryRunRequest{
		MigrationId: "mig",
		Candidate:   candidate,
		Steps: []api.StepDefinition{
			{Name: "disable-base-resource-prune", MigratorApp: "app-chart-migrator", Type: strPtr("disable-base-resource-prune")},
		},
	}

	result, err := r.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)

	step := result.Steps[0]
	assert.False(t, step.Skipped)
	require.NotEmpty(t, *step.Files)

	diff := (*step.Files)[0]
	assert.Equal(t, "apps/my-app/base.yaml", diff.Path)
	assert.True(t, strings.Contains(diff.After, "Prune=false"), "expected Prune=false annotation in output")
}

// TestRunner_StackedDiffs verifies that steps see file content modified by earlier steps,
// not the original GitHub state.
func TestRunner_StackedDiffs(t *testing.T) {
	gh := githubadapter.NewInMem()
	gh.SetFile(gitopsOwner, gitopsRepo, "apps/my-app/base.yaml",
		"apiVersion: argoproj.io/v1alpha1\nkind: Deployment\nmetadata:\n  name: my-app\n")

	r := &dryrun.Runner{RealClient: gh, StepCfg: stepCfg()}

	candidate := api.Candidate{
		Id: "my-app",
		Files: &[]api.FileGroup{
			{
				Name:  "base",
				Files: []api.FileRef{{Path: "apps/my-app/base.yaml"}},
			},
		},
	}

	// Two identical steps — second one should see the already-annotated file.
	req := api.DryRunRequest{
		MigrationId: "mig",
		Candidate:   candidate,
		Steps: []api.StepDefinition{
			{Name: "step-1", MigratorApp: "app-chart-migrator", Type: strPtr("disable-base-resource-prune")},
			{Name: "step-2", MigratorApp: "app-chart-migrator", Type: strPtr("disable-base-resource-prune")},
		},
	}

	result, err := r.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, result.Steps, 2)

	// Both steps should produce output for the same file.
	assert.False(t, result.Steps[0].Skipped)
	assert.False(t, result.Steps[1].Skipped)

	// The "before" content of step 2 should be the "after" content of step 1.
	step1After := (*result.Steps[0].Files)[0].After
	step2Before := *(*result.Steps[1].Files)[0].Before
	assert.Equal(t, step1After, step2Before, "step 2 should see step 1's output as its before content")
}
