// Package github provides factory functions for creating authenticated GitHub
// API clients. Callers should use the returned *github.Client with the adapter
// in apps/worker/internal/adapters/github to interact with GitHub repositories.
package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gogithub "github.com/google/go-github/v75/github"
	"golang.org/x/oauth2"
)

const defaultAPIURL = "https://api.github.com"

// NewTokenClient creates a *github.Client authenticated with a personal access token.
// Used for local development. Pass baseURL="" to use the real GitHub API, or a
// custom URL (e.g. "http://localhost:9090") for a mock server.
func NewTokenClient(token, baseURL string) *gogithub.Client {
	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = oauth2.NewClient(context.Background(), ts)
	}
	c := gogithub.NewClient(httpClient)
	applyBaseURL(c, baseURL)
	return c
}

// NewAppClient creates a *github.Client authenticated as a GitHub App installation.
// Used in deployed environments. privateKeyPath is the path to the app's PEM private key.
func NewAppClient(appID, installationID int64, privateKeyPath, baseURL string) (*gogithub.Client, error) {
	base := baseURL
	if base == "" {
		base = defaultAPIURL
	}

	tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installationID, privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("github app auth: %w", err)
	}
	tr.BaseURL = base

	c := gogithub.NewClient(&http.Client{Transport: tr})
	applyBaseURL(c, baseURL)
	return c, nil
}

func applyBaseURL(c *gogithub.Client, baseURL string) {
	if baseURL == "" || baseURL == defaultAPIURL {
		return
	}
	u, err := url.Parse(baseURL + "/")
	if err != nil {
		return
	}
	c.BaseURL = u
}
