package migrations

import (
	"fmt"
	"time"

	"github.com/tilsley/loom/pkg/api"
)

// TargetAlreadyRunError is returned when a target repo has already been run or is currently running.
type TargetAlreadyRunError struct {
	Repo   string
	Status string
}

// Error implements the error interface.
func (e TargetAlreadyRunError) Error() string {
	return fmt.Sprintf("target %q already has status %q", e.Repo, e.Status)
}

// WorkflowStatus is the port-level representation returned by the WorkflowEngine.
type WorkflowStatus struct {
	RuntimeStatus string
	Output        []byte // Raw JSON of the workflow result (if finished)
}

// MigrationStatus is the service-level view of a running or finished migration.
type MigrationStatus struct {
	InstanceID    string
	RuntimeStatus string
	Result        *api.MigrationResult
}

// StepEventName returns the deterministic event name the workflow waits on
// for a given step+target combination. Workers receive this in DispatchStepRequest.EventName
// and the workflow listens for it via WaitForExternalEvent.
func StepEventName(stepName string, target api.Target) string {
	return fmt.Sprintf("step-completed:%s:%s", stepName, target.Repo)
}

// PROpenedEventName returns the deterministic signal name the workflow listens on
// for intermediate PR-opened notifications for a given step+target combination.
func PROpenedEventName(stepName string, target api.Target) string {
	return fmt.Sprintf("pr-opened:%s:%s", stepName, target.Repo)
}

// GenerateRunID creates a human-readable, sortable run ID from a migration ID.
func GenerateRunID(migrationID string) string {
	return fmt.Sprintf("%s-%d", migrationID, time.Now().Unix())
}
