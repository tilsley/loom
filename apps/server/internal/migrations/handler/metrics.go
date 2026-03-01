package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MetricsOverview returns aggregate totals for the metrics dashboard.
func (h *Handler) MetricsOverview(c *gin.Context) {
	overview, err := h.svc.GetMetricsOverview(c.Request.Context())
	if err != nil {
		h.log.Error("metrics overview failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics overview"})
		return
	}
	c.JSON(http.StatusOK, overview)
}

// MetricsSteps returns per-step-name aggregated statistics.
func (h *Handler) MetricsSteps(c *gin.Context) {
	steps, err := h.svc.GetStepMetrics(c.Request.Context())
	if err != nil {
		h.log.Error("metrics steps failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch step metrics"})
		return
	}
	c.JSON(http.StatusOK, steps)
}

// MetricsTimeline returns daily event counts for the specified number of days.
func (h *Handler) MetricsTimeline(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	timeline, err := h.svc.GetMetricsTimeline(c.Request.Context(), days)
	if err != nil {
		h.log.Error("metrics timeline failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch timeline"})
		return
	}
	c.JSON(http.StatusOK, timeline)
}

// MetricsFailures returns recent failed step events.
func (h *Handler) MetricsFailures(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	failures, err := h.svc.GetRecentFailures(c.Request.Context(), limit)
	if err != nil {
		h.log.Error("metrics failures failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch failures"})
		return
	}
	c.JSON(http.StatusOK, failures)
}
