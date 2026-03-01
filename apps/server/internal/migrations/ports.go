package migrations

import (
	"context"
	"time"

	"github.com/tilsley/loom/pkg/api"
)

// Event type constants for the EventStore.
const (
	EventStepDispatched = "step_dispatched"
	EventStepCompleted  = "step_completed"
	EventStepRetried    = "step_retried"
	EventRunStarted     = "run_started"
	EventRunCompleted   = "run_completed"
	EventRunCancelled   = "run_cancelled"
)

// StepEvent represents a lifecycle event recorded into the event store.
type StepEvent struct {
	ID          int64             `json:"id"`
	MigrationID string            `json:"migrationId"`
	CandidateID string            `json:"candidateId"`
	StepName    string            `json:"stepName,omitempty"`
	EventType   string            `json:"eventType"`
	Status      string            `json:"status,omitempty"`
	DurationMs  *int              `json:"durationMs,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// MetricsOverview holds aggregate totals for the metrics dashboard.
type MetricsOverview struct {
	TotalRuns     int     `json:"totalRuns"`
	CompletedRuns int     `json:"completedRuns"`
	FailedSteps   int     `json:"failedSteps"`
	PRsRaised     int     `json:"prsRaised"`
	AvgDurationMs float64 `json:"avgDurationMs"`
	FailureRate   float64 `json:"failureRate"`
}

// StepMetrics holds per-step-name aggregated statistics.
type StepMetrics struct {
	StepName    string  `json:"stepName"`
	Count       int     `json:"count"`
	AvgMs       float64 `json:"avgMs"`
	P95Ms       float64 `json:"p95Ms"`
	FailureRate float64 `json:"failureRate"`
}

// TimelinePoint holds daily event counts for charting.
type TimelinePoint struct {
	Date      string `json:"date"`
	Started   int    `json:"started"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
}

// EventStore records and queries workflow lifecycle events.
type EventStore interface {
	RecordEvent(ctx context.Context, event StepEvent) error
	GetOverview(ctx context.Context) (*MetricsOverview, error)
	GetStepMetrics(ctx context.Context) ([]StepMetrics, error)
	GetTimeline(ctx context.Context, days int) ([]TimelinePoint, error)
	GetRecentFailures(ctx context.Context, limit int) ([]StepEvent, error)
}

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
	UpdateCandidateMetadata(ctx context.Context, migrationID, candidateID string, metadata map[string]string) error
}
