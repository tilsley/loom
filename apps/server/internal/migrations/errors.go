package migrations

import "fmt"

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
