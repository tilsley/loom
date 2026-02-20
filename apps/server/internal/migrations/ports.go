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
}

// MigrationStore persists registered migration definitions.
type MigrationStore interface {
	Save(ctx context.Context, m api.RegisteredMigration) error
	Get(ctx context.Context, id string) (*api.RegisteredMigration, error)
	List(ctx context.Context) ([]api.RegisteredMigration, error)
	Delete(ctx context.Context, id string) error
	AppendRunID(ctx context.Context, id string, runID string) error
	SetTargetRun(ctx context.Context, migrationID, targetRepo string, run api.TargetRun) error
}
