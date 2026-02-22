package migrations

import (
	"context"

	"github.com/tilsley/loom/pkg/api"
)

// WorkerNotifier dispatches step requests to external worker applications.
// Implementations live in the adapters layer (e.g. Dapr pub/sub, HTTP).
type WorkerNotifier interface {
	Dispatch(ctx context.Context, req api.DispatchStepRequest) error
}

// WorkflowEngine abstracts the durable workflow runtime.
// The platform/dapr layer provides the concrete implementation.
type WorkflowEngine interface {
	StartWorkflow(ctx context.Context, workflowName, instanceID string, input any) (string, error)
	GetStatus(ctx context.Context, instanceID string) (*WorkflowStatus, error)
	RaiseEvent(ctx context.Context, instanceID, eventName string, payload any) error
	CancelWorkflow(ctx context.Context, instanceID string) error
}

// DryRunner simulates a full migration run and returns per-step file diffs.
// Implementations call a worker via service invocation (e.g. Dapr).
type DryRunner interface {
	DryRun(ctx context.Context, req api.DryRunRequest) (*api.DryRunResult, error)
}

// MigrationStore persists registered migration definitions.
type MigrationStore interface {
	Save(ctx context.Context, m api.RegisteredMigration) error
	Get(ctx context.Context, id string) (*api.RegisteredMigration, error)
	List(ctx context.Context) ([]api.RegisteredMigration, error)
	Delete(ctx context.Context, id string) error
	AppendRunID(ctx context.Context, id string, runID string) error
	AppendCancelledAttempt(ctx context.Context, migrationID string, attempt api.CancelledAttempt) error
	SetCandidateRun(ctx context.Context, migrationID, candidateID string, run api.CandidateRun) error
	DeleteCandidateRun(ctx context.Context, migrationID, candidateID string) error
	SaveCandidates(ctx context.Context, migrationID string, candidates []api.Candidate) error
	GetCandidates(ctx context.Context, migrationID string) ([]api.CandidateWithStatus, error)
	StoreRunRecord(ctx context.Context, runId string, record RunRecord) error
	GetRunRecord(ctx context.Context, runId string) (*RunRecord, error)
	DeleteRunRecord(ctx context.Context, runId string) error
}
