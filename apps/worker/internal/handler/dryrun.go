package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/apps/worker/internal/dryrun"
	"github.com/tilsley/loom/pkg/api"
)

// DryRun handles POST /dry-run — runs all migration steps in simulation mode
// and returns per-step file diffs without creating any real pull requests.
type DryRun struct {
	runner *dryrun.Runner
	log    *slog.Logger
}

// NewDryRun creates a DryRun handler.
func NewDryRun(runner *dryrun.Runner, log *slog.Logger) *DryRun {
	return &DryRun{runner: runner, log: log}
}

// Handle processes a DryRunRequest and returns per-step file diffs.
func (h *DryRun) Handle(c *gin.Context) {
	var req api.DryRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("dry run started", "migrationId", req.MigrationId, "candidate", req.Candidate.Id, "steps", len(req.Steps))

	result, err := h.runner.Run(c.Request.Context(), req)
	if err != nil {
		h.log.Error("dry run failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
