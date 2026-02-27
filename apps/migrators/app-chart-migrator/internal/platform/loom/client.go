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
func (c *Client) SendCallback(ctx context.Context, callbackID string, event api.StepStatusEvent) error {
	url := fmt.Sprintf("%s/event/%s", c.BaseURL, callbackID)
	return c.post(ctx, url, event)
}

// SendUpdate signals the workflow that a step is still in progress, carrying
// arbitrary metadata for UI rendering (e.g. prUrl, instructions).
// The workflow continues waiting for a subsequent terminal signal.
func (c *Client) SendUpdate(ctx context.Context, callbackID, stepName, candidateID string, metadata map[string]string) {
	event := api.StepStatusEvent{
		StepName:    stepName,
		CandidateId: candidateID,
		Status:      api.StepStatusEventStatusPending,
	}
	if len(metadata) > 0 {
		event.Metadata = &metadata
	}
	if err := c.SendCallback(ctx, callbackID, event); err != nil {
		c.Log.Warn("failed to send update", "error", err, "callbackId", callbackID)
	}
}

func (c *Client) post(ctx context.Context, url string, event api.StepStatusEvent) error {
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
