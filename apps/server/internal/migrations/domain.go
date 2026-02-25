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

// RunNotFoundError is returned by the ExecutionEngine when the run instance
// does not exist — typically after the engine is restarted in development.
type RunNotFoundError struct {
	InstanceID string
}

// Error implements the error interface.
func (e RunNotFoundError) Error() string {
	return fmt.Sprintf("run %q not found", e.InstanceID)
}

// RunStatus is the port-level representation returned by the ExecutionEngine.
type RunStatus struct {
	RuntimeStatus string
	Output        []byte // Raw JSON of the run result (if finished)
}

// StepEventName returns the deterministic signal name the run listens on
// for a given step+candidate combination. Workers receive this in DispatchStepRequest.EventName.
func StepEventName(stepName, candidateId string) string {
	return fmt.Sprintf("step-completed:%s:%s", stepName, candidateId)
}

// RetryStepEventName returns the deterministic signal name the run listens on
// when waiting for an operator to retry a failed step.
func RetryStepEventName(stepName, candidateId string) string {
	return fmt.Sprintf("retry-step:%s:%s", stepName, candidateId)
}

const runIDSep = "__"

// RunID returns the deterministic run instance ID for a migration+candidate pair.
// Since each candidate runs at most once per migration, the ID is stable and recoverable.
func RunID(migrationID, candidateID string) string {
	return migrationID + runIDSep + candidateID
}

// ParseRunID splits a run ID back into its migrationID and candidateID components.
func ParseRunID(runID string) (migrationID, candidateID string, err error) {
	parts := strings.SplitN(runID, runIDSep, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid run ID %q: expected format <migrationId>_<candidateId>", runID)
	}
	return parts[0], parts[1], nil
}
