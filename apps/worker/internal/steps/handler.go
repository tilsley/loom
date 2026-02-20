package steps

import (
	"context"
	"strings"

	"github.com/tilsley/loom/apps/worker/internal/github"
	"github.com/tilsley/loom/pkg/api"
)

// Handler is the interface that each migration step type implements.
type Handler interface {
	Execute(ctx context.Context, gh *github.Client, cfg *Config, req api.DispatchStepRequest) (*Result, error)
}

// Result describes the PR to create after a step handler completes.
type Result struct {
	Owner  string
	Repo   string
	Title  string
	Body   string
	Branch string
	Files  map[string]string // path → new content
}

// appName returns the app name for a target. If target.Metadata["appName"]
// is set, it is used directly; otherwise we fall back to parsing target.Repo.
func appName(target api.Target) string {
	if target.Metadata != nil {
		if name, ok := (*target.Metadata)["appName"]; ok && name != "" {
			return name
		}
	}
	parts := strings.SplitN(target.Repo, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return target.Repo
}

// targetOwnerRepo splits target.Repo "acme/billing-api" into ("acme", "billing-api").
func targetOwnerRepo(target api.Target) (string, string) {
	parts := strings.SplitN(target.Repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return target.Repo, ""
}
