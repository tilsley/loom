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

// SendProgress posts an intermediate progress event to /event/{callbackID}/pr-opened.
func (c *Client) SendProgress(ctx context.Context, callbackID string, event api.StepCompletedEvent) error {
	url := fmt.Sprintf("%s/event/%s/pr-opened", c.BaseURL, callbackID)
	return c.post(ctx, url, event)
}

// SendCallback posts a completion event to /event/{callbackID}.
func (c *Client) SendCallback(ctx context.Context, callbackID string, event api.StepCompletedEvent) error {
	url := fmt.Sprintf("%s/event/%s", c.BaseURL, callbackID)
	return c.post(ctx, url, event)
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
