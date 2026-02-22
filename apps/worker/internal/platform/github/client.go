package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
)

// Client talks to the GitHub (or mock-GitHub) API.
// It implicitly satisfies gitrepo.Client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a GitHub API client pointing at baseURL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// CreatePR creates a pull request in the given repository.
func (c *Client) CreatePR(ctx context.Context, owner, repo string, req gitrepo.CreatePRRequest) (*gitrepo.PullRequest, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal PR request: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.baseURL, owner, repo)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer func() { //nolint:errcheck // response body close errors are non-actionable after reading
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("POST %s returned %d", url, resp.StatusCode)
	}

	var pr gitrepo.PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode PR response: %w", err)
	}
	return &pr, nil
}

// ListDir returns the immediate children of dirPath from the GitHub contents API.
func (c *Client) ListDir(ctx context.Context, owner, repo, dirPath string) ([]gitrepo.DirEntry, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, dirPath)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer func() { //nolint:errcheck // response body close errors are non-actionable after reading
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", url, resp.StatusCode)
	}

	var entries []gitrepo.DirEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode dir listing: %w", err)
	}
	return entries, nil
}

// GetContents fetches a file from the GitHub contents API and decodes it.
func (c *Client) GetContents(ctx context.Context, owner, repo, path string) (*gitrepo.FileContent, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, path)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer func() { //nolint:errcheck // response body close errors are non-actionable after reading
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", url, resp.StatusCode)
	}

	var raw struct {
		Path     string `json:"path"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(raw.Content)
	if err != nil {
		return nil, fmt.Errorf("decode base64 content for %s: %w", path, err)
	}

	return &gitrepo.FileContent{
		Path:    raw.Path,
		Content: string(decoded),
	}, nil
}
