package migrations

import (
	"fmt"
	"strings"

	"github.com/tilsley/loom/pkg/api"
)

// RuntimeStatus values returned by the ExecutionEngine.
const (
	RuntimeStatusRunning   = "RUNNING"
	RuntimeStatusCompleted = "COMPLETED"
	RuntimeStatusFailed    = "FAILED"
	RuntimeStatusUnknown   = "UNKNOWN"
)

// RunStatus is the port-level representation returned by the ExecutionEngine.
type RunStatus struct {
	RuntimeStatus string
	Steps         []api.StepState // Step results from the run; populated for both running and completed runs.
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

// UpdateInputsEventName returns the signal name used to push updated metadata
// into a running workflow for the given candidate.
func UpdateInputsEventName(candidateId string) string {
	return fmt.Sprintf("update-inputs:%s", candidateId)
}
