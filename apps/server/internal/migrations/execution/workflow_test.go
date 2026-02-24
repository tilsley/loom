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
	return execution.NewActivities(nil, nil, slog.Default())
}

// dummyMigrator configures env so that every DispatchStep call immediately signals
// step-completed back to the workflow, simulating a worker that succeeds instantly.
func dummyMigrator(env *testsuite.TestWorkflowEnvironment, acts *execution.Activities) {
	env.OnActivity(acts.DispatchStep, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(api.DispatchStepRequest)
			env.RegisterDelayedCallback(func() {
				env.SignalWorkflow(req.EventName, api.StepCompletedEvent{
					StepName:    req.StepName,
					CandidateId: req.Candidate.Id,
					Success:     true,
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
		Steps:       []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
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
	require.True(t, result.Results[0].Success)
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
			{Name: "update-chart", WorkerApp: "app-chart-migrator"},
			{Name: "open-pr", WorkerApp: "app-chart-migrator"},
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
		Steps:       []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
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
	retrySent := false
	env.OnActivity(acts.DispatchStep, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(api.DispatchStepRequest)
			env.RegisterDelayedCallback(func() {
				if !retrySent {
					env.SignalWorkflow(req.EventName, api.StepCompletedEvent{
						StepName:    req.StepName,
						CandidateId: req.Candidate.Id,
						Success:     false,
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
					env.SignalWorkflow(req.EventName, api.StepCompletedEvent{
						StepName:    req.StepName,
						CandidateId: req.Candidate.Id,
						Success:     true,
					})
				}
			}, time.Millisecond)
		})

	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{{Id: "billing-api"}},
		Steps:       []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Success)
}

// ─── Manual review step ───────────────────────────────────────────────────────

func TestMigrationOrchestrator_ManualReviewStep_SkipsDispatch(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	// DispatchStep should NOT be called for a manual-review step.
	dispatched := false
	env.OnActivity(acts.DispatchStep, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(_ mock.Arguments) { dispatched = true })

	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	candidate := api.Candidate{Id: "billing-api"}

	// Provide the step-completed signal manually (no dispatch → we send it directly).
	stepSignal := migrations.StepEventName("review", candidate.Id)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(stepSignal, api.StepCompletedEvent{
			StepName:    "review",
			CandidateId: candidate.Id,
			Success:     true,
		})
	}, time.Millisecond)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{candidate},
		Steps: []api.StepDefinition{
			{
				Name:      "review",
				WorkerApp: "app-chart-migrator",
				Config:    &map[string]string{"type": "manual-review", "instructions": "approve the PR"},
			},
		},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.False(t, dispatched, "DispatchStep must not be called for manual-review steps")

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
}

// ─── PR opened signal ─────────────────────────────────────────────────────────

func TestMigrationOrchestrator_PROpened_MetadataCaptured(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	acts := newActivities()
	env.RegisterActivity(acts)

	candidate := api.Candidate{Id: "billing-api"}
	prURL := "https://github.com/owner/repo/pull/42"

	env.OnActivity(acts.DispatchStep, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(api.DispatchStepRequest)

			// Send pr-opened first, then step-completed.
			prSignal := migrations.PROpenedEventName(req.StepName, candidate.Id)
			env.RegisterDelayedCallback(func() {
				env.SignalWorkflow(prSignal, api.StepCompletedEvent{
					StepName:    req.StepName,
					CandidateId: candidate.Id,
					Success:     true,
					Metadata:    &map[string]string{"prUrl": prURL},
				})
			}, time.Millisecond)

			env.RegisterDelayedCallback(func() {
				env.SignalWorkflow(req.EventName, api.StepCompletedEvent{
					StepName:    req.StepName,
					CandidateId: candidate.Id,
					Success:     true,
					Metadata:    &map[string]string{"prUrl": prURL},
				})
			}, 2*time.Millisecond)
		})

	env.OnActivity(acts.UpdateCandidateStatus, mock.Anything, mock.Anything).Return(nil)

	manifest := api.MigrationManifest{
		MigrationId: "mig-abc",
		Candidates:  []api.Candidate{candidate},
		Steps:       []api.StepDefinition{{Name: "open-pr", WorkerApp: "app-chart-migrator"}},
	}

	env.ExecuteWorkflow(execution.MigrationOrchestrator, manifest)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result execution.MigrationResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "completed", result.Status)
	require.Len(t, result.Results, 1)
	require.NotNil(t, result.Results[0].Metadata)
	require.Equal(t, prURL, (*result.Results[0].Metadata)["prUrl"])
}
