package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/pkg/api"
)

// Event handles POST /event/:id — worker callback that unblocks a waiting step in an active run.
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

// Announce handles POST /registry/announce — migrators POST a MigrationAnnouncement directly.
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

	h.log.Info("migration announced", "id", m.Id, "name", m.Name, "migratorUrl", announcement.MigratorUrl)
	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
}
