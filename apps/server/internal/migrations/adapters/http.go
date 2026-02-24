package adapters

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// Handler translates HTTP requests into calls on the migrations.Service.
type Handler struct {
	svc *migrations.Service
	log *slog.Logger
}

// RegisterRoutes mounts the Loom migration API onto the given Gin engine.
func RegisterRoutes(r *gin.Engine, svc *migrations.Service, log *slog.Logger) {
	h := &Handler{svc: svc, log: log}

	r.POST("/event/:id", h.Event)
	r.POST("/event/:id/pr-opened", h.PROpened)

	r.POST("/registry/announce", h.Announce)

	// Migrations
	r.GET("/migrations", h.List)
	r.GET("/migrations/:id", h.GetMigration)
	r.POST("/migrations/:id/candidates", h.SubmitCandidates)
	r.GET("/migrations/:id/candidates", h.GetCandidates)
	r.POST("/migrations/:id/dry-run", h.DryRun)

	// Candidate lifecycle (candidate ID in URL)
	r.POST("/migrations/:id/candidates/:candidateId/start", h.StartRun)
	r.POST("/migrations/:id/candidates/:candidateId/cancel", h.CancelRun)
	r.POST("/migrations/:id/candidates/:candidateId/retry-step", h.RetryStep)
	r.GET("/migrations/:id/candidates/:candidateId/steps", h.GetCandidateSteps)
}

// Event handles POST /event/:id — worker callback endpoint to resume a paused workflow step.
func (h *Handler) Event(c *gin.Context) {
	id := c.Param("id")

	var event api.StepCompletedEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.HandleEvent(c.Request.Context(), id, event); err != nil {
		h.log.Error("failed to handle event", "instanceId", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "event raised"})
}

// List handles GET /migrations — lists all migrations.
func (h *Handler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.log.Error("failed to list migrations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.ListMigrationsResponse{Migrations: items})
}

// GetMigration handles GET /migrations/:id — gets a specific migration.
func (h *Handler) GetMigration(c *gin.Context) {
	id := c.Param("id")

	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.log.Error("failed to get migration", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "migration not found"})
		return
	}

	c.JSON(http.StatusOK, m)
}

// StartRun handles POST /migrations/:id/candidates/:candidateId/start —
// starts the migration workflow for the given candidate.
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
// cancels the running workflow, records the attempt, and resets the candidate to not_started.
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
// raises a retry-step signal into the running workflow, re-dispatching the named step.
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
		c.JSON(http.StatusNotFound, gin.H{"error": "no active or completed workflow found for this candidate"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// PROpened handles POST /event/:id/pr-opened — worker notifies that a PR has been created.
// Signals the workflow so the PR URL is immediately queryable by the console.
func (h *Handler) PROpened(c *gin.Context) {
	id := c.Param("id")

	var event api.StepCompletedEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.HandlePROpened(c.Request.Context(), id, event); err != nil {
		h.log.Error("failed to handle PR opened", "instanceId", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "pr recorded"})
}

// SubmitCandidates handles POST /migrations/:id/candidates — worker submits discovered candidates.
func (h *Handler) SubmitCandidates(c *gin.Context) {
	id := c.Param("id")

	var req api.SubmitCandidatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.SubmitCandidates(c.Request.Context(), id, req); err != nil {
		var migNotFound migrations.MigrationNotFoundError
		if errors.As(err, &migNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		h.log.Error("failed to submit candidates", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetCandidates handles GET /migrations/:id/candidates — console fetches candidates with status.
func (h *Handler) GetCandidates(c *gin.Context) {
	id := c.Param("id")

	candidates, err := h.svc.GetCandidates(c.Request.Context(), id)
	if err != nil {
		h.log.Error("failed to get candidates", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, candidates)
}

// DryRun handles POST /migrations/:id/dry-run — simulates the migration run for a candidate.
func (h *Handler) DryRun(c *gin.Context) {
	id := c.Param("id")

	var req api.DryRunCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.DryRun(c.Request.Context(), id, req.Candidate)
	if err != nil {
		var migNotFound migrations.MigrationNotFoundError
		if errors.As(err, &migNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		h.log.Error("dry run failed", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Announce handles POST /registry/announce — workers POST a MigrationAnnouncement directly.
func (h *Handler) Announce(c *gin.Context) {
	var announcement api.MigrationAnnouncement
	if err := c.ShouldBindJSON(&announcement); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.svc.Announce(c.Request.Context(), announcement)
	if err != nil {
		h.log.Error("failed to handle announcement", "id", announcement.Id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("migration announced", "id", m.Id, "name", m.Name, "workerUrl", announcement.WorkerUrl)
	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
}
