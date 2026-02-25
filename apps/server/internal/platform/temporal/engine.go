package temporalplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/tilsley/loom/apps/server/internal/migrations"
)

// Compile-time check: *Engine implements migrations.ExecutionEngine.
var _ migrations.ExecutionEngine = (*Engine)(nil)

const taskQueue = "loom-migrations"

// Engine implements migrations.ExecutionEngine using the Temporal SDK client.
type Engine struct {
	c client.Client
}

// NewEngine creates a new Temporal workflow engine.
func NewEngine(c client.Client) *Engine {
	return &Engine{c: c}
}

// TaskQueue returns the Temporal task queue name used by the engine.
func TaskQueue() string { return taskQueue }

// StartRun starts a new Temporal workflow execution.
func (e *Engine) StartRun(ctx context.Context, name, instanceID string, input any) (string, error) {
	opts := client.StartWorkflowOptions{
		ID:        instanceID,
		TaskQueue: taskQueue,
	}
	run, err := e.c.ExecuteWorkflow(ctx, opts, name, input)
	if err != nil {
		return "", fmt.Errorf("start run %q: %w", name, err)
	}
	return run.GetID(), nil
}

// GetStatus returns the current status of a workflow execution, including live progress.
func (e *Engine) GetStatus(ctx context.Context, instanceID string) (*migrations.RunStatus, error) {
	desc, err := e.c.DescribeWorkflowExecution(ctx, instanceID, "")
	if err != nil {
		if isNotFound(err) {
			return nil, migrations.RunNotFoundError{InstanceID: instanceID}
		}
		return nil, fmt.Errorf("describe workflow %q: %w", instanceID, err)
	}

	status := mapTemporalStatus(desc.WorkflowExecutionInfo.Status)
	ws := &migrations.RunStatus{
		RuntimeStatus: status,
	}

	// For running workflows, query for live progress.
	if status == "RUNNING" {
		val, err := e.c.QueryWorkflow(ctx, instanceID, "", "progress")
		if err == nil {
			var results json.RawMessage
			if err := val.Get(&results); err == nil {
				ws.Output = results
			}
		}
		return ws, nil
	}

	// For completed/failed workflows, get the final result.
	run := e.c.GetWorkflow(ctx, instanceID, "")
	var result json.RawMessage
	if err := run.Get(ctx, &result); err == nil {
		ws.Output = result
	}

	return ws, nil
}

// RaiseEvent signals a running workflow with an external event.
func (e *Engine) RaiseEvent(ctx context.Context, instanceID, eventName string, payload any) error {
	if err := e.c.SignalWorkflow(ctx, instanceID, "", eventName, payload); err != nil {
		return fmt.Errorf("signal %q on %q: %w", eventName, instanceID, err)
	}
	return nil
}

// CancelRun requests graceful cancellation of a running workflow.
// The workflow's ctx.Done() channel becomes readable, allowing any blocking
// Selector (including awaitStepCompletion and awaitRetryOrCancel) to unblock
// and return, after which the workflow completes in a Cancelled terminal state.
func (e *Engine) CancelRun(ctx context.Context, instanceID string) error {
	if err := e.c.CancelWorkflow(ctx, instanceID, ""); err != nil {
		return fmt.Errorf("cancel run %q: %w", instanceID, err)
	}
	return nil
}

// isNotFound reports whether err indicates a workflow execution was not found.
// Temporal wraps gRPC NOT_FOUND errors; checking the message is the portable approach.
func isNotFound(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist")
}

func mapTemporalStatus(s enumspb.WorkflowExecutionStatus) string {
	switch s {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return migrations.RuntimeStatusRunning
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return migrations.RuntimeStatusCompleted
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED,
		enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return migrations.RuntimeStatusFailed
	case enumspb.WORKFLOW_EXECUTION_STATUS_UNSPECIFIED,
		enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return migrations.RuntimeStatusUnknown
	default:
		return migrations.RuntimeStatusUnknown
	}
}
