package migrations

import (
	"fmt"
	"strings"
)

// CandidateAlreadyRunError is returned when a candidate already has a running or completed run.
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
