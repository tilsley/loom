package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/apps/worker/internal/pending"
	"github.com/tilsley/loom/pkg/api"
)

// WebhookPayload matches the shape of a GitHub pull_request webhook event.
type WebhookPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number  int    `json:"number"`
		Merged  bool   `json:"merged"`
		HTMLURL string `json:"html_url"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// Webhook handles GitHub webhook events (real or mock).
type Webhook struct {
	pending *pending.Store
	loomURL string
	log     *slog.Logger
}

// NewWebhook creates a Webhook handler.
func NewWebhook(store *pending.Store, loomURL string, log *slog.Logger) *Webhook {
	return &Webhook{pending: store, loomURL: loomURL, log: log}
}

// Handle processes a GitHub pull_request webhook event.
func (w *Webhook) Handle(c *gin.Context) {
	var payload WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only care about merged PRs.
	if payload.Action != "closed" || !payload.PullRequest.Merged {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	key := fmt.Sprintf("%s#%d", payload.Repository.FullName, payload.PullRequest.Number)
	cb, ok := w.pending.Remove(key)
	if !ok {
		w.log.Warn("no pending callback for merged PR", "key", key)
		c.JSON(http.StatusOK, gin.H{"status": "no pending callback"})
		return
	}

	w.log.Info("PR merged, sending callback", "target", cb.Target.Repo, "step", cb.StepName, "pr", cb.PRURL)

	meta := map[string]string{
		"phase":     "merged",
		"prUrl":     cb.PRURL,
		"commitSha": fmt.Sprintf("sha-merged-%s-%d", cb.StepName, payload.PullRequest.Number),
	}

	event := api.StepCompletedEvent{
		StepName: cb.StepName,
		Target:   cb.Target,
		Success:  true,
		Metadata: &meta,
	}

	body, err := json.Marshal(event)
	if err != nil {
		w.log.Error("failed to marshal callback", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal failed"})
		return
	}

	url := fmt.Sprintf("%s/event/%s", w.loomURL, cb.CallbackID)
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		w.log.Error("failed to create callback request", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "callback failed"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		w.log.Error("failed to send callback", "error", err, "url", url)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "callback failed"})
		return
	}
	defer func() { //nolint:errcheck // response body close errors are non-actionable after reading
		_ = resp.Body.Close()
	}()

	w.log.Info("callback sent", "url", url, "status", resp.StatusCode)
	c.JSON(http.StatusOK, gin.H{"status": "callback sent"})
}
