package steps

import (
	"context"
	"strings"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/pkg/api"
)

// Handler is the interface that each migration step type implements.
type Handler interface {
	Execute(ctx context.Context, gr gitrepo.Client, cfg *Config, req api.DispatchStepRequest) (*Result, error)
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

// appName returns the logical name for a candidate. Uses candidate.Id directly,
// which is the logical app/service name set by the discoverer.
func appName(candidate api.Candidate) string {
	return candidate.Id
}

// gitopsFileForEnv returns the actual path of the ArgoCD Application manifest
// for the given environment, looked up from the candidate's discovered FileGroups.
// The discoverer names each gitops FileGroup after the environment it found the
// file in (e.g. "dev", "staging", "prod"), so we can match by group name.
// Returns ("", false) if no file is found for that environment.
func gitopsFileForEnv(candidate api.Candidate, env string) (string, bool) {
	if candidate.Files == nil {
		return "", false
	}
	for _, group := range *candidate.Files {
		if group.Name == env && len(group.Files) > 0 {
			return group.Files[0].Path, true
		}
	}
	return "", false
}

// candidateOwnerRepo splits the repoName from candidate metadata into ("owner", "repo").
// The discoverer is expected to set metadata["repoName"] = "owner/repo".
func candidateOwnerRepo(candidate api.Candidate) (string, string) {
	if candidate.Metadata != nil {
		if repoName, ok := (*candidate.Metadata)["repoName"]; ok && repoName != "" {
			parts := strings.SplitN(repoName, "/", 2)
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
		}
	}
	// Fallback: treat ID itself as owner/repo if it contains a slash.
	parts := strings.SplitN(candidate.Id, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return candidate.Id, ""
}
