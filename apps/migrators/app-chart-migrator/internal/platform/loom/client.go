package loom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/tilsley/loom/pkg/api"
)

// Client sends progress and callback events to the Loom server.
type Client struct {
	BaseURL string
	Log     *slog.Logger
}

// NewClient creates a Loom HTTP client.
func NewClient(baseURL string, log *slog.Logger) *Client {
	return &Client{BaseURL: baseURL, Log: log}
}

// SendCallback posts a completion event to /event/{callbackID}.
func (c *Client) SendCallback(ctx context.Context, callbackID string, event api.StepCompletedEvent) error {
	url := fmt.Sprintf("%s/event/%s", c.BaseURL, callbackID)
	return c.post(ctx, url, event)
}

// NotifyPROpened signals the workflow that a PR has been opened for the step,
// transitioning its visible status to "open" while it awaits merge.
// The workflow continues waiting for a subsequent step-completed signal.
func (c *Client) NotifyPROpened(ctx context.Context, callbackID, stepName, candidateID, prURL string) {
	event := api.StepCompletedEvent{
		StepName:    stepName,
		CandidateId: candidateID,
		Success:     true,
		Metadata:    &map[string]string{"phase": "open", "prUrl": prURL},
	}
	if err := c.SendCallback(ctx, callbackID, event); err != nil {
		c.Log.Warn("failed to notify PR opened", "error", err, "callbackId", callbackID)
	}
}

// NotifyAwaitingReview signals the workflow that a manual-review step is ready
// for operator action, transitioning its visible status to "awaiting_review".
// An optional instructions string is surfaced in the UI review panel.
// The workflow continues waiting for a subsequent approve/reject signal.
func (c *Client) NotifyAwaitingReview(ctx context.Context, callbackID, stepName, candidateID, instructions string) {
	meta := map[string]string{"phase": "awaiting_review"}
	if instructions != "" {
		meta["instructions"] = instructions
	}
	event := api.StepCompletedEvent{
		StepName:    stepName,
		CandidateId: candidateID,
		Success:     true,
		Metadata:    &meta,
	}
	if err := c.SendCallback(ctx, callbackID, event); err != nil {
		c.Log.Warn("failed to notify awaiting review", "error", err, "callbackId", callbackID)
	}
}

func (c *Client) post(ctx context.Context, url string, event api.StepCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer func() { //nolint:errcheck // response body close errors are non-actionable after reading
		_ = resp.Body.Close()
	}()

	return nil
}
