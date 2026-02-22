package discovery

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/pkg/api"
)

// legacyRepoURLFragment identifies ArgoCD Applications still using the old generic chart.
const legacyRepoURLFragment = "charts.example.com/generic"

// argoApplication is a minimal struct for parsing ArgoCD Application manifests.
type argoApplication struct {
	Spec struct {
		Source struct {
			RepoURL        string `yaml:"repoURL"`
			Chart          string `yaml:"chart"`
			TargetRevision string `yaml:"targetRevision"`
		} `yaml:"source"`
	} `yaml:"spec"`
}

// AppChartDiscoverer finds ArgoCD apps in the GitOps repo that need
// the app-chart migration applied.
//
// Discovery logic:
//  1. List app directories under apps/ in the GitOps repo
//  2. For each app, fetch apps/{app}/base/application.yaml
//  3. Parse the ArgoCD Application manifest
//  4. If spec.source.repoURL matches the legacy generic chart, emit a candidate
//     with file groups derived from the actual overlay structure
type AppChartDiscoverer struct {
	GH          gitrepo.Client
	GitopsOwner string
	GitopsRepo  string
}

// Discover scans the GitOps repo and returns ArgoCD apps that need migration.
func (d *AppChartDiscoverer) Discover(ctx context.Context) ([]api.Candidate, error) {
	entries, err := d.GH.ListDir(ctx, d.GitopsOwner, d.GitopsRepo, "apps")
	if err != nil {
		return nil, fmt.Errorf("list apps dir: %w", err)
	}

	var candidates []api.Candidate
	for _, entry := range entries {
		if entry.Type != "dir" {
			continue
		}
		appName := entry.Name

		basePath := fmt.Sprintf("apps/%s/base/application.yaml", appName)
		file, err := d.GH.GetContents(ctx, d.GitopsOwner, d.GitopsRepo, basePath)
		if err != nil {
			// App exists in the directory listing but has no base application.yaml — skip it.
			continue
		}

		var app argoApplication
		if err := yaml.Unmarshal([]byte(file.Content), &app); err != nil {
			continue
		}

		if !strings.Contains(app.Spec.Source.RepoURL, legacyRepoURLFragment) {
			continue
		}

		fileGroups := d.buildFileGroups(ctx, appName)

		kind := "application"
		candidates = append(candidates, api.Candidate{
			Id:   appName,
			Kind: &kind,
			Metadata: &map[string]string{
				"repoName":   d.GitopsOwner + "/" + appName,
				"gitopsPath": fmt.Sprintf("apps/%s", appName),
			},
			State: &map[string]string{
				"currentChart":   app.Spec.Source.Chart,
				"currentRepoURL": app.Spec.Source.RepoURL,
				"currentVersion": app.Spec.Source.TargetRevision,
			},
			Files: &fileGroups,
		})
	}

	return candidates, nil
}

// buildFileGroups enumerates the actual overlay environments for appName and
// returns the complete set of file groups that the migration will touch.
// Errors listing overlays are silently ignored — we fall back to base + app-repo only.
func (d *AppChartDiscoverer) buildFileGroups(ctx context.Context, appName string) []api.FileGroup {
	gitopsRepo := d.GitopsOwner + "/" + d.GitopsRepo
	appRepo := d.GitopsOwner + "/" + appName
	gitopsBase := fmt.Sprintf("https://github.com/%s/%s/blob/main", d.GitopsOwner, d.GitopsRepo)
	appRepoBase := fmt.Sprintf("https://github.com/%s/%s/blob/main", d.GitopsOwner, appName)

	// Discover actual environments from the overlays directory.
	var envs []string
	overlayEntries, err := d.GH.ListDir(ctx, d.GitopsOwner, d.GitopsRepo, fmt.Sprintf("apps/%s/overlays", appName))
	if err == nil {
		for _, e := range overlayEntries {
			if e.Type == "dir" {
				envs = append(envs, e.Name)
			}
		}
	}

	ref := func(base, path string) api.FileRef {
		return api.FileRef{Path: path, Url: base + "/" + path}
	}

	var groups []api.FileGroup

	// Base group — GitOps repo files shared across all environments.
	groups = append(groups, api.FileGroup{
		Name: "base",
		Repo: gitopsRepo,
		Files: []api.FileRef{
			ref(gitopsBase, fmt.Sprintf("apps/%s/base/service-monitor.yaml", appName)),
			ref(gitopsBase, fmt.Sprintf("apps/%s/base/application.yaml", appName)),
		},
	})

	// Per-environment groups — one overlay file each.
	for _, env := range envs {
		groups = append(groups, api.FileGroup{
			Name: env,
			Repo: gitopsRepo,
			Files: []api.FileRef{
				ref(gitopsBase, fmt.Sprintf("apps/%s/overlays/%s/application.yaml", appName, env)),
			},
		})
	}

	// App-repo group — only existing files that the migration will modify.
	// Files created from scratch (Chart.yaml, values.yaml, publish-chart.yaml, etc.)
	// are omitted here; they will appear in dry-run output as new-file diffs.
	groups = append(groups, api.FileGroup{
		Name:  "app-repo",
		Repo:  appRepo,
		Files: []api.FileRef{ref(appRepoBase, ".github/workflows/ci.yaml")},
	})

	return groups
}
