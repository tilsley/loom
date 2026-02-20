package adapters

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

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

	// Legacy workflow endpoints
	r.POST("/start", h.Start)
	r.GET("/status/:id", h.Status)
	r.POST("/event/:id", h.Event)
	r.POST("/event/:id/pr-opened", h.PROpened)

	// Dapr pub/sub subscription discovery + handler
	r.GET("/dapr/subscribe", h.DaprSubscribe)
	r.POST("/registry/announce", h.Announce)

	// Registered migrations CRUD + run
	r.POST("/migrations", h.Register)
	r.GET("/migrations", h.List)
	r.GET("/migrations/:id", h.GetMigration)
	r.DELETE("/migrations/:id", h.DeleteMigration)
	r.POST("/migrations/:id/run", h.RunMigration)
}

// Start handles POST /start — initiates a migration workflow from a manifest.
func (h *Handler) Start(c *gin.Context) {
	var req api.StartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := h.svc.Start(c.Request.Context(), req.Manifest)
	if err != nil {
		h.log.Error("failed to start migration", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"instanceId": id})
}

// Status handles GET /status/:id — queries workflow state and all accumulated metadata / PR links.
func (h *Handler) Status(c *gin.Context) {
	id := c.Param("id")

	status, err := h.svc.Status(c.Request.Context(), id)
	if err != nil {
		h.log.Error("failed to get status", "instanceId", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"instanceId":    status.InstanceID,
		"runtimeStatus": status.RuntimeStatus,
		"result":        status.Result,
	})
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

// Register handles POST /migrations — registers a new migration definition.
func (h *Handler) Register(c *gin.Context) {
	var req api.RegisterMigrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		h.log.Error("failed to register migration", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, m)
}

// List handles GET /migrations — lists all registered migrations.
func (h *Handler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.log.Error("failed to list migrations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.ListMigrationsResponse{Migrations: items})
}

// GetMigration handles GET /migrations/:id — gets a specific registered migration.
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

// DeleteMigration handles DELETE /migrations/:id — removes a registered migration.
func (h *Handler) DeleteMigration(c *gin.Context) {
	id := c.Param("id")

	// Check existence first.
	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.log.Error("failed to get migration for delete", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "migration not found"})
		return
	}

	if err := h.svc.DeleteMigration(c.Request.Context(), id); err != nil {
		h.log.Error("failed to delete migration", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// RunMigration handles POST /migrations/:id/run — starts a workflow run for a single target.
func (h *Handler) RunMigration(c *gin.Context) {
	id := c.Param("id")

	var req api.RunMigrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	runID, err := h.svc.Run(c.Request.Context(), id, req.Target)
	if err != nil {
		var alreadyRun migrations.TargetAlreadyRunError
		if errors.As(err, &alreadyRun) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		h.log.Error("failed to run migration", "id", id, "target", req.Target.Repo, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, api.RunMigrationResponse{RunId: runID})
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

// DaprSubscribe handles GET /dapr/subscribe — Dapr calls this to discover pub/sub subscriptions.
func (h *Handler) DaprSubscribe(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{
			"pubsubname": "pubsub",
			"topic":      "migration-registry",
			"route":      "/registry/announce",
		},
	})
}

// Announce handles POST /registry/announce — Dapr delivers pub/sub messages here (CloudEvent envelope).
func (h *Handler) Announce(c *gin.Context) {
	var envelope struct {
		Data api.MigrationAnnouncement `json:"data"`
	}
	if err := c.ShouldBindJSON(&envelope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.svc.Announce(c.Request.Context(), envelope.Data)
	if err != nil {
		h.log.Error("failed to handle announcement", "id", envelope.Data.Id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("migration announced", "id", m.Id, "name", m.Name)
	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
}
