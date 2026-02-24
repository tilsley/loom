package migrations

import (
	"fmt"
	"strings"
)

// MigrationNotFoundError is returned when the requested migration does not exist in the store.
type MigrationNotFoundError struct {
	ID string
}

// Error implements the error interface.
func (e MigrationNotFoundError) Error() string {
	return fmt.Sprintf("migration %q not found", e.ID)
}

// CandidateNotFoundError is returned when the requested candidate does not exist in the migration.
type CandidateNotFoundError struct {
	MigrationID string
	CandidateID string
}

// Error implements the error interface.
func (e CandidateNotFoundError) Error() string {
	return fmt.Sprintf("candidate %q not found in migration %q", e.CandidateID, e.MigrationID)
}

// CandidateAlreadyRunError is returned when a candidate already has a running or completed run.
type CandidateAlreadyRunError struct {
	ID     string
	Status string
}

// Error implements the error interface.
func (e CandidateAlreadyRunError) Error() string {
	return fmt.Sprintf("candidate %q already has status %q", e.ID, e.Status)
}

// CandidateNotRunningError is returned when cancel is requested for a candidate that is not running.
type CandidateNotRunningError struct {
	ID string
}

// Error implements the error interface.
func (e CandidateNotRunningError) Error() string {
	return fmt.Sprintf("candidate %q is not running", e.ID)
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

// StepEventName returns the deterministic event name the workflow waits on
// for a given step+candidate combination. Workers receive this in DispatchStepRequest.EventName
// and the workflow listens for it via WaitForExternalEvent.
func StepEventName(stepName, candidateId string) string {
	return fmt.Sprintf("step-completed:%s:%s", stepName, candidateId)
}

// PROpenedEventName returns the deterministic signal name the workflow listens on
// for intermediate PR-opened notifications for a given step+candidate combination.
func PROpenedEventName(stepName, candidateId string) string {
	return fmt.Sprintf("pr-opened:%s:%s", stepName, candidateId)
}

// RetryStepEventName returns the deterministic signal name the workflow listens on
// when waiting for an operator to retry a failed step.
func RetryStepEventName(stepName, candidateId string) string {
	return fmt.Sprintf("retry-step:%s:%s", stepName, candidateId)
}

const runIDSep = "__"

// WorkflowID returns the deterministic Temporal workflow instance ID for a migration+candidate pair.
// Since each candidate runs at most once per migration, the ID is stable and recoverable.
func WorkflowID(migrationID, candidateID string) string {
	return migrationID + runIDSep + candidateID
}

// ParseWorkflowID splits a workflow ID back into its migrationID and candidateID components.
func ParseWorkflowID(workflowID string) (migrationID, candidateID string, err error) {
	parts := strings.SplitN(workflowID, runIDSep, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid workflow ID %q: expected format <migrationId>_<candidateId>", workflowID)
	}
	return parts[0], parts[1], nil
}
