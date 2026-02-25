package migrations

import (
	"context"

	"github.com/tilsley/loom/pkg/api"
)

// MigratorNotifier dispatches step requests to external migrators.
// Implementations live in the adapters layer (e.g. Dapr pub/sub, HTTP).
type MigratorNotifier interface {
	Dispatch(ctx context.Context, req api.DispatchStepRequest) error
}

// ExecutionEngine abstracts the durable execution runtime.
type ExecutionEngine interface {
	StartRun(ctx context.Context, runType, instanceID string, input any) (string, error)
	GetStatus(ctx context.Context, instanceID string) (*RunStatus, error)
	RaiseEvent(ctx context.Context, instanceID, eventName string, payload any) error
	CancelRun(ctx context.Context, instanceID string) error
}

// DryRunner simulates a full migration run and returns per-step file diffs.
type DryRunner interface {
	DryRun(ctx context.Context, migratorUrl string, req api.DryRunRequest) (*api.DryRunResult, error)
}

// MigrationStore persists migration definitions.
type MigrationStore interface {
	Save(ctx context.Context, m api.Migration) error
	Get(ctx context.Context, id string) (*api.Migration, error)
	List(ctx context.Context) ([]api.Migration, error)
	SetCandidateStatus(ctx context.Context, migrationID, candidateID string, status api.CandidateStatus) error
	SaveCandidates(ctx context.Context, migrationID string, candidates []api.Candidate) error
	GetCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error)
}
