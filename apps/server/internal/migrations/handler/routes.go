package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/apps/server/internal/migrations"
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
