package migrations

import (
	"fmt"
	"time"

	"github.com/tilsley/loom/pkg/api"
)

// CandidateAlreadyRunError is returned when a candidate repo already has a queued, running, or completed run.
type CandidateAlreadyRunError struct {
	ID     string
	Status string
}

// Error implements the error interface.
func (e CandidateAlreadyRunError) Error() string {
	return fmt.Sprintf("candidate %q already has status %q", e.ID, e.Status)
}

// WorkflowNotFoundError is returned by the WorkflowEngine when the workflow instance
// does not exist — typically after the engine is restarted in development.
type WorkflowNotFoundError struct {
	InstanceID string
}

// Error implements the error interface.
func (e WorkflowNotFoundError) Error() string {
	return fmt.Sprintf("workflow %q not found", e.InstanceID)
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
// for a given step+candidate combination. Workers receive this in DispatchStepRequest.EventName
// and the workflow listens for it via WaitForExternalEvent.
func StepEventName(stepName string, candidate api.Candidate) string {
	return fmt.Sprintf("step-completed:%s:%s", stepName, candidate.Id)
}

// PROpenedEventName returns the deterministic signal name the workflow listens on
// for intermediate PR-opened notifications for a given step+candidate combination.
func PROpenedEventName(stepName string, candidate api.Candidate) string {
	return fmt.Sprintf("pr-opened:%s:%s", stepName, candidate.Id)
}

// GenerateRunID creates a human-readable, sortable run ID from a migration ID.
func GenerateRunID(migrationID string) string {
	return fmt.Sprintf("%s-%d", migrationID, time.Now().Unix())
}

// RunRecord holds the information needed to execute a queued run.
// It is stored in the state store and looked up when Execute is called.
type RunRecord struct {
	MigrationID string        `json:"migrationId"`
	Candidate   api.Candidate `json:"candidate"`
}
