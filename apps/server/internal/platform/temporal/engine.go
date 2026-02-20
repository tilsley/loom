package temporalplatform

import (
	"context"
	"encoding/json"
	"fmt"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/tilsley/loom/apps/server/internal/migrations"
)

// Compile-time check: *Engine implements migrations.WorkflowEngine.
var _ migrations.WorkflowEngine = (*Engine)(nil)

const (
	taskQueue    = "loom-migrations"
	statusFailed = "FAILED"
)

// Engine implements migrations.WorkflowEngine using the Temporal SDK client.
type Engine struct {
	c client.Client
}

// NewEngine creates a new Temporal workflow engine.
func NewEngine(c client.Client) *Engine {
	return &Engine{c: c}
}

// TaskQueue returns the Temporal task queue name used by the engine.
func TaskQueue() string { return taskQueue }

// StartWorkflow starts a new Temporal workflow execution.
func (e *Engine) StartWorkflow(ctx context.Context, name, instanceID string, input any) (string, error) {
	opts := client.StartWorkflowOptions{
		ID:        instanceID,
		TaskQueue: taskQueue,
	}
	run, err := e.c.ExecuteWorkflow(ctx, opts, name, input)
	if err != nil {
		return "", fmt.Errorf("start workflow %q: %w", name, err)
	}
	return run.GetID(), nil
}

// GetStatus returns the current status of a workflow execution, including live progress.
func (e *Engine) GetStatus(ctx context.Context, instanceID string) (*migrations.WorkflowStatus, error) {
	desc, err := e.c.DescribeWorkflowExecution(ctx, instanceID, "")
	if err != nil {
		return nil, fmt.Errorf("describe workflow %q: %w", instanceID, err)
	}

	status := mapTemporalStatus(desc.WorkflowExecutionInfo.Status)
	ws := &migrations.WorkflowStatus{
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

func mapTemporalStatus(s enumspb.WorkflowExecutionStatus) string {
	switch s {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return "RUNNING"
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return "COMPLETED"
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED,
		enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return statusFailed
	case enumspb.WORKFLOW_EXECUTION_STATUS_UNSPECIFIED,
		enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}
