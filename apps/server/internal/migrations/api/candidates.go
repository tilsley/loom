package handlers

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// StartRun handles POST /migrations/:id/candidates/:candidateId/start —
// starts a run for the given candidate.
func (h *Handler) StartRun(c *gin.Context) {
	id := c.Param("id")
	candidateID := c.Param("candidateId")

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.String("migration.id", id),
		attribute.String("candidate.id", candidateID),
	)

	// Body is optional — only contains inputs when the migration has requiredInputs.
	var req api.StartRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var inputs map[string]string
	if req.Inputs != nil {
		inputs = *req.Inputs
	}

	_, err := h.svc.Start(c.Request.Context(), id, candidateID, inputs)
	if err != nil {
		var alreadyRun migrations.CandidateAlreadyRunError
		if errors.As(err, &alreadyRun) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		var migNotFound migrations.MigrationNotFoundError
		var candNotFound migrations.CandidateNotFoundError
		if errors.As(err, &migNotFound) || errors.As(err, &candNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		h.log.Error("failed to start run", "id", id, "candidateId", candidateID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusAccepted)
}

// CancelRun handles POST /migrations/:id/candidates/:candidateId/cancel —
// cancels the active run and resets the candidate to not_started.
func (h *Handler) CancelRun(c *gin.Context) {
	id := c.Param("id")
	candidateID := c.Param("candidateId")

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.String("migration.id", id),
		attribute.String("candidate.id", candidateID),
	)

	if err := h.svc.Cancel(c.Request.Context(), id, candidateID); err != nil {
		var notRunning migrations.CandidateNotRunningError
		if errors.As(err, &notRunning) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		var migNotFound migrations.MigrationNotFoundError
		var candNotFound migrations.CandidateNotFoundError
		if errors.As(err, &migNotFound) || errors.As(err, &candNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		h.log.Error("failed to cancel run", "id", id, "candidateId", candidateID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// RetryStep handles POST /migrations/:id/candidates/:candidateId/retry-step —
// raises a retry-step signal into the active run, re-dispatching the named step.
func (h *Handler) RetryStep(c *gin.Context) {
	id := c.Param("id")
	candidateID := c.Param("candidateId")

	var req api.RetryStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.RetryStep(c.Request.Context(), id, candidateID, req.StepName); err != nil {
		var notRunning migrations.CandidateNotRunningError
		if errors.As(err, &notRunning) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		var migNotFound migrations.MigrationNotFoundError
		var candNotFound migrations.CandidateNotFoundError
		if errors.As(err, &migNotFound) || errors.As(err, &candNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		h.log.Error("failed to retry step", "id", id, "candidateId", candidateID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusAccepted)
}

// GetCandidateSteps handles GET /migrations/:id/candidates/:candidateId/steps —
// returns step execution progress for a running or completed candidate.
func (h *Handler) GetCandidateSteps(c *gin.Context) {
	id := c.Param("id")
	candidateID := c.Param("candidateId")

	resp, err := h.svc.GetCandidateSteps(c.Request.Context(), id, candidateID)
	if err != nil {
		h.log.Error("failed to get candidate steps", "id", id, "candidateId", candidateID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if resp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active or completed run found for this candidate"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
