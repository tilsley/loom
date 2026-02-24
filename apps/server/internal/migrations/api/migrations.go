package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

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
