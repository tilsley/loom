package execution_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/apps/server/internal/migrations/execution"
	"github.com/tilsley/loom/pkg/api"
)

// newActivities returns an Activities instance suitable for use in workflow tests.
// All activity methods are mocked via env.OnActivity so the nil dependencies
// are never actually called.
func newActivities() *execution.Activities {
	return execution.NewActivities(nil, nil, nil, slog.Default())
}

// dummyMigrator configures env so that every DispatchStep call immediately signals
// step-completed back to the workflow, simulating a worker that succeeds instantly.
func dummyMigrator(env *testsuite.TestWorkflowEnvironment, acts *execution.Activities) {
	env.OnActivity(acts.RecordEvent, mock.Anything, mock.Anything).Return(nil).Maybe()
	env.OnActivity(acts.DispatchStep, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(api.DispatchStepRequest)
			env.RegisterDelayedCallback(func() {
				env.SignalWorkflow(req.EventName, api.StepStatusEvent{
					StepName:    req.StepName,
					CandidateId: req.Candidate.Id,
					Status:      api.StepStatusEventStatusSucceeded,
				})
			}, time.Millisecond)
		})
}

// ─── Happy path ───────────────────────────────────────────────────────────────

func TestMigrationOrchestrator_SingleStep_Success(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	dummyMigrator(env, acts)
	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{{Id: "billing-api"}},
		Steps:       []api.StepDefinition{{Name: "update-chart", MigratorApp: "app-chart-migrator"}},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
	require.Len(t, result.Results, 1)
	require.Equal(t, "update-chart", result.Results[0].StepName)
	require.Equal(t, "billing-api", result.Results[0].Candidate.Id)
	require.Equal(t, api.StepStateStatusSucceeded, result.Results[0].Status)
}

func TestMigrationOrchestrator_MultiStep_Success(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	dummyMigrator(env, acts)
	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{{Id: "billing-api"}},
		Steps: []api.StepDefinition{
			{Name: "update-chart", MigratorApp: "app-chart-migrator"},
			{Name: "open-pr", MigratorApp: "app-chart-migrator"},
		},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
	require.Len(t, result.Results, 2)
	require.Equal(t, "update-chart", result.Results[0].StepName)
	require.Equal(t, "open-pr", result.Results[1].StepName)
}

func TestMigrationOrchestrator_MultiCandidate_Success(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	dummyMigrator(env, acts)
	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{{Id: "billing-api"}, {Id: "payments-svc"}},
		Steps:       []api.StepDefinition{{Name: "update-chart", MigratorApp: "app-chart-migrator"}},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
	require.Len(t, result.Results, 2)

	ids := []string{result.Results[0].Candidate.Id, result.Results[1].Candidate.Id}
	require.Contains(t, ids, "billing-api")
	require.Contains(t, ids, "payments-svc")
}

// ─── Failure + retry ─────────────────────────────────────────────────────────

func TestMigrationOrchestrator_StepFailure_RetrySucceeds(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	// First dispatch signals failure; a retry signal is then sent; second dispatch succeeds.
	env.OnActivity(acts.RecordEvent, mock.Anything, mock.Anything).Return(nil).Maybe()
	retrySent := false
	env.OnActivity(acts.DispatchStep, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(api.DispatchStepRequest)
			env.RegisterDelayedCallback(func() {
				if !retrySent {
					env.SignalWorkflow(req.EventName, api.StepStatusEvent{
						StepName:    req.StepName,
						CandidateId: req.Candidate.Id,
						Status:      api.StepStatusEventStatusFailed,
					})
					// After the workflow receives the failure and starts waiting, send retry.
					env.RegisterDelayedCallback(func() {
						retrySent = true
						env.SignalWorkflow(
							migrations.RetryStepEventName(req.StepName, req.Candidate.Id),
							nil,
						)
					}, time.Millisecond)
				} else {
					env.SignalWorkflow(req.EventName, api.StepStatusEvent{
						StepName:    req.StepName,
						CandidateId: req.Candidate.Id,
						Status:      api.StepStatusEventStatusSucceeded,
					})
				}
			}, time.Millisecond)
		})

	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{{Id: "billing-api"}},
		Steps:       []api.StepDefinition{{Name: "update-chart", MigratorApp: "app-chart-migrator"}},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
	require.Len(t, result.Results, 1)
	require.Equal(t, api.StepStateStatusSucceeded, result.Results[0].Status)
}

// ─── Manual review step ───────────────────────────────────────────────────────

// TestMigrationOrchestrator_ManualReviewStep_DispatchedToWorker verifies that a
// manual-review step is dispatched to the worker like any other step. The worker
// is responsible for signalling awaiting_review and the operator completes it
// via the standard event endpoint.
func TestMigrationOrchestrator_ManualReviewStep_DispatchedToWorker(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	dummyMigrator(env, acts)
	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{{Id: "billing-api"}},
		Steps: []api.StepDefinition{
			{
				Name:      "review",
				MigratorApp: "app-chart-migrator",
				Config:    &map[string]string{"type": "manual-review", "instructions": "approve the PR"},
			},
		},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
	require.Len(t, result.Results, 1)
	require.Equal(t, api.StepStateStatusSucceeded, result.Results[0].Status)
}

// ─── Update inputs signal ─────────────────────────────────────────────────────

// TestMigrationOrchestrator_UpdateInputs_AppliedOnRetry verifies that metadata
// sent via the update-inputs signal is used in the next DispatchStep call.
func TestMigrationOrchestrator_UpdateInputs_AppliedOnRetry(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	// Track the DispatchStepRequests so we can inspect metadata on retry.
	env.OnActivity(acts.RecordEvent, mock.Anything, mock.Anything).Return(nil).Maybe()
	var dispatches []api.DispatchStepRequest
	retrySent := false

	env.OnActivity(acts.DispatchStep, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(api.DispatchStepRequest)
			dispatches = append(dispatches, req)

			env.RegisterDelayedCallback(func() {
				if !retrySent {
					// First dispatch → signal failure.
					env.SignalWorkflow(req.EventName, api.StepStatusEvent{
						StepName:    req.StepName,
						CandidateId: req.Candidate.Id,
						Status:      api.StepStatusEventStatusFailed,
					})
					// While waiting for retry, push updated inputs then send retry.
					env.RegisterDelayedCallback(func() {
						env.SignalWorkflow(
							migrations.UpdateInputsEventName(req.Candidate.Id),
							map[string]string{"repoName": "fixed-name"},
						)
						env.RegisterDelayedCallback(func() {
							retrySent = true
							env.SignalWorkflow(
								migrations.RetryStepEventName(req.StepName, req.Candidate.Id),
								nil,
							)
						}, time.Millisecond)
					}, time.Millisecond)
				} else {
					// Second dispatch (after retry) → succeed.
					env.SignalWorkflow(req.EventName, api.StepStatusEvent{
						StepName:    req.StepName,
						CandidateId: req.Candidate.Id,
						Status:      api.StepStatusEventStatusSucceeded,
					})
				}
			}, time.Millisecond)
		})

	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	md := map[string]string{"repoName": "typo-name"}
	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{{Id: "billing-api", Metadata: &md}},
		Steps:       []api.StepDefinition{{Name: "update-chart", MigratorApp: "app-chart-migrator"}},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Should have been dispatched twice (initial + retry).
	require.Len(t, dispatches, 2)

	// First dispatch should have the original metadata.
	require.NotNil(t, dispatches[0].Candidate.Metadata)
	require.Equal(t, "typo-name", (*dispatches[0].Candidate.Metadata)["repoName"])

	// Second dispatch (after update-inputs signal) should have the corrected metadata.
	require.NotNil(t, dispatches[1].Candidate.Metadata)
	require.Equal(t, "fixed-name", (*dispatches[1].Candidate.Metadata)["repoName"])

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
}

