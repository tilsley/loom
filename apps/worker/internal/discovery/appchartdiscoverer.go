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
	Kind     string `yaml:"kind"`
	Metadata struct {
		Labels map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Spec struct {
		Source struct {
			RepoURL        string `yaml:"repoURL"`
			Chart          string `yaml:"chart"`
			TargetRevision string `yaml:"targetRevision"`
		} `yaml:"source"`
	} `yaml:"spec"`
}

// fileEntry holds a discovered YAML file path alongside its parsed manifest.
type fileEntry struct {
	path string
	app  argoApplication
}

// AppChartDiscoverer finds ArgoCD apps in the GitOps repo that need
// the app-chart migration applied.
//
// Discovery logic:
//  1. Download the entire GitOps repo in one shot via RepoReader.ReadAll.
//  2. Parse every .yaml/.yml file; skip on parse error, wrong kind, or missing label.
//  3. Filter to files where spec.source.repoURL contains the legacy chart fragment.
//  4. Group matching files by their app.kubernetes.io/instance label —
//     each unique label becomes one candidate.
//  5. Emit a Candidate per group with gitops + app-repo file groups.
type AppChartDiscoverer struct {
	Reader      gitrepo.RepoReader
	GitopsOwner string
	GitopsRepo  string
}

// Discover downloads the GitOps repo and returns ArgoCD apps that need migration.
func (d *AppChartDiscoverer) Discover(ctx context.Context) ([]api.Candidate, error) {
	allFiles, err := d.Reader.ReadAll(ctx, d.GitopsOwner, d.GitopsRepo)
	if err != nil {
		return nil, fmt.Errorf("read gitops repo: %w", err)
	}

	// byInstance groups matching file entries by app.kubernetes.io/instance label.
	byInstance := make(map[string][]fileEntry)

	for path, content := range allFiles {
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			continue
		}

		var app argoApplication
		if err := yaml.Unmarshal([]byte(content), &app); err != nil {
			continue
		}
		if app.Kind != "Application" {
			continue
		}
		if !strings.Contains(app.Spec.Source.RepoURL, legacyRepoURLFragment) {
			continue
		}

		instance := app.Metadata.Labels["app.kubernetes.io/instance"]
		if instance == "" {
			continue
		}

		byInstance[instance] = append(byInstance[instance], fileEntry{path: path, app: app})
	}

	gitopsRepo := d.GitopsOwner + "/" + d.GitopsRepo
	gitopsBase := fmt.Sprintf("https://github.com/%s/%s/blob/main", d.GitopsOwner, d.GitopsRepo)

	var candidates []api.Candidate
	for instance, entries := range byInstance {
		// Group discovered files by environment, extracted from path position 3:
		// src / <team> / <system> / <ENV> / <cloud> / <namespace> / <file>.yaml
		envFiles := make(map[string][]api.FileRef)
		for _, e := range entries {
			parts := strings.Split(e.path, "/")
			env := "unknown"
			if len(parts) >= 4 {
				env = parts[3]
			}
			envFiles[env] = append(envFiles[env], api.FileRef{
				Path: e.path,
				Url:  gitopsBase + "/" + e.path,
			})
		}

		fileGroups := make([]api.FileGroup, 0, len(envFiles)+1)
		for env, files := range envFiles {
			fileGroups = append(fileGroups, api.FileGroup{
				Name:  env,
				Repo:  gitopsRepo,
				Files: files,
			})
		}

		appRepo := d.GitopsOwner + "/" + instance
		appRepoBase := fmt.Sprintf("https://github.com/%s/%s/blob/main", d.GitopsOwner, instance)
		fileGroups = append(fileGroups, api.FileGroup{
			Name: "app-repo",
			Repo: appRepo,
			Files: []api.FileRef{
				{Path: ".github/workflows/ci.yaml", Url: appRepoBase + "/.github/workflows/ci.yaml"},
			},
		})

		first := entries[0].app
		kind := "application"
		candidates = append(candidates, api.Candidate{
			Id:   instance,
			Kind: &kind,
			Metadata: &map[string]string{
				"repoName": appRepo,
			},
			State: &map[string]string{
				"currentChart":   first.Spec.Source.Chart,
				"currentRepoURL": first.Spec.Source.RepoURL,
				"currentVersion": first.Spec.Source.TargetRevision,
			},
			Files: &fileGroups,
		})
	}

	return candidates, nil
}
