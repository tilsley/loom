package dryrun

import (
	"context"

	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/steps"
	"github.com/tilsley/loom/pkg/api"
)

// Runner iterates all steps in a DryRunRequest, executing each registered
// step handler against a RecordingClient. Steps for other worker apps and
// unregistered step types are marked as skipped. No PRs are created.
type Runner struct {
	RealClient gitrepo.Client
	StepCfg    *steps.Config
}

// Run executes the dry run and returns per-step file diffs.
// Steps are executed in order with stacked diffs: each step sees the accumulated
// output of all previous steps, so the "before" content for any given file
// reflects the state after all prior steps have been applied, not the live
// GitHub state. This matches how the real migration runs (each PR is merged
// before the next step starts).
func (r *Runner) Run(ctx context.Context, req api.DryRunRequest) (*api.DryRunResult, error) {
	var stepResults []api.StepDryRunResult

	// overlay accumulates every file written by executed steps.
	// Keyed by "owner/repo/path" — same format used by RecordingClient.
	overlay := make(map[string]string)

	for _, stepDef := range req.Steps {
		// Skip steps handled by other worker apps.
		if stepDef.MigratorApp != "app-chart-migrator" {
			stepResults = append(stepResults, api.StepDryRunResult{
				StepName: stepDef.Name,
				Skipped:  true,
			})
			continue
		}

		// Skip steps with no type.
		if stepDef.Type == nil {
			stepResults = append(stepResults, api.StepDryRunResult{
				StepName: stepDef.Name,
				Skipped:  true,
			})
			continue
		}

		h, found := steps.Lookup(*stepDef.Type)
		if !found {
			stepResults = append(stepResults, api.StepDryRunResult{
				StepName: stepDef.Name,
				Skipped:  true,
			})
			continue
		}

		rec := gitrepo.NewRecordingClient(r.RealClient, overlay)

		dispatchReq := api.DispatchStepRequest{
			MigrationId: req.MigrationId,
			StepName:    stepDef.Name,
			Candidate:   req.Candidate,
			Config:      stepDef.Config,
			Type:        stepDef.Type,
			CallbackId:  "dry-run",
			EventName:   "dry-run",
		}

		result, err := h.Execute(ctx, rec, r.StepCfg, dispatchReq)
		if err != nil {
			errStr := err.Error()
			stepResults = append(stepResults, api.StepDryRunResult{
				StepName: stepDef.Name,
				Skipped:  false,
				Error:    &errStr,
			})
			continue
		}

		var fileDiffs []api.FileDiff
		if result != nil {
			for path, after := range result.Files {
				before := rec.ContentBefore(result.Owner, result.Repo, path)
				status := api.Modified
				if before == "" {
					status = api.New
				} else if after == "" {
					status = api.Deleted
				}
				fileDiffs = append(fileDiffs, api.FileDiff{
					Path:   path,
					Repo:   result.Repo,
					Before: &before,
					After:  after,
					Status: status,
				})
				// Accumulate this step's output so subsequent steps see the
				// updated file content rather than the original GitHub state.
				overlay[result.Owner+"/"+result.Repo+"/"+path] = after
			}
		}

		stepResults = append(stepResults, api.StepDryRunResult{
			StepName: stepDef.Name,
			Skipped:  false,
			Files:    &fileDiffs,
		})
	}

	return &api.DryRunResult{Steps: stepResults}, nil
}
