package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
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

// k8sResource is a minimal struct for identifying Kubernetes resources.
type k8sResource struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

// isBaseResource reports whether content is a patchable Kubernetes resource
// (has apiVersion + kind, is not an ArgoCD Application or Kustomize config).
func isBaseResource(content string) bool {
	var r k8sResource
	if err := yaml.Unmarshal([]byte(content), &r); err != nil {
		return false
	}
	if r.APIVersion == "" || r.Kind == "" {
		return false
	}
	switch r.Kind {
	case "Application", "Kustomization":
		return false
	}
	return true
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
//
// Envs is the ordered list of all configured environments (e.g. ["dev","staging","prod"]).
// StepBuilder, if set, is called with the subset of Envs the candidate was found in,
// returning the tailored step list stored on the candidate. This lets each candidate
// carry only the steps that apply to it rather than the full migration template.
type AppChartDiscoverer struct {
	Reader      gitrepo.RepoReader
	GitopsOwner string
	GitopsRepo  string
	Envs        []string
	StepBuilder func(envs []string) []api.StepDefinition
	Log         *slog.Logger
}

// Discover downloads the GitOps repo and returns ArgoCD apps that need migration.
func (d *AppChartDiscoverer) Discover(ctx context.Context) ([]api.Candidate, error) {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}

	log.Info("fetching repo snapshot", "owner", d.GitopsOwner, "repo", d.GitopsRepo)

	allFiles, err := d.Reader.ReadAll(ctx, d.GitopsOwner, d.GitopsRepo)
	if err != nil {
		return nil, fmt.Errorf("read gitops repo: %w", err)
	}

	log.Info("repo snapshot fetched", "files", len(allFiles))

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

	gitopsRepo := d.GitopsRepo
	gitopsBase := fmt.Sprintf("https://github.com/%s/%s/blob/main", d.GitopsOwner, d.GitopsRepo)

	// Build per-instance metadata needed for the second pass.
	// prefix: "src/<team>/<system>" derived from each Application file path.
	type instanceMeta struct {
		prefix   string
		envs     map[string]struct{} // parts[3] values that had Application CRDs
		appPaths map[string]struct{} // paths already captured as Application CRDs
	}
	meta := make(map[string]*instanceMeta)
	for instance, entries := range byInstance {
		prefix := ""
		envs := make(map[string]struct{})
		appPaths := make(map[string]struct{})
		for _, e := range entries {
			parts := strings.Split(e.path, "/")
			if len(parts) >= 3 && prefix == "" {
				prefix = strings.Join(parts[:3], "/")
			}
			if len(parts) >= 4 {
				envs[parts[3]] = struct{}{}
			}
			appPaths[e.path] = struct{}{}
		}
		meta[instance] = &instanceMeta{prefix: prefix, envs: envs, appPaths: appPaths}
	}

	// Build a reverse map: prefix → instance, so the second pass can find the owner.
	prefixToInstance := make(map[string]string, len(meta))
	for instance, m := range meta {
		if m.prefix != "" {
			prefixToInstance[m.prefix] = instance
		}
	}

	// Second pass: collect non-Application YAML files under each candidate's prefix.
	// Files at a path segment (parts[3]) that is NOT one of the app's known envs
	// are "base/common" resources — grouped under "base" in the candidate's Files.
	baseFiles := make(map[string][]api.FileRef) // instance → []FileRef
	for path := range allFiles {
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			continue
		}
		parts := strings.Split(path, "/")
		if len(parts) < 4 {
			continue
		}
		prefix := strings.Join(parts[:3], "/")
		instance, ok := prefixToInstance[prefix]
		if !ok {
			continue
		}
		m := meta[instance]
		if _, isAppPath := m.appPaths[path]; isAppPath {
			continue // already captured as an Application CRD
		}
		if _, isEnv := m.envs[parts[3]]; isEnv {
			continue // env-specific non-Application file; leave to env steps
		}
		if !isBaseResource(allFiles[path]) {
			continue // not a patchable Kubernetes resource (e.g. kustomization.yaml)
		}
		baseFiles[instance] = append(baseFiles[instance], api.FileRef{
			Path: path,
			Url:  gitopsBase + "/" + path,
		})
	}

	var candidates []api.Candidate
	for instance, entries := range byInstance {
		// Group Application CRDs by environment (parts[3]).
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

		fileGroups := make([]api.FileGroup, 0, len(envFiles)+2)
		for env, files := range envFiles {
			fileGroups = append(fileGroups, api.FileGroup{
				Name:  env,
				Repo:  gitopsRepo,
				Files: files,
			})
		}

		// Add "base" group for non-Application resources in common/base dirs.
		if bf := baseFiles[instance]; len(bf) > 0 {
			fileGroups = append(fileGroups, api.FileGroup{
				Name:  "base",
				Repo:  gitopsRepo,
				Files: bf,
			})
			log.Debug("discovered base files", "candidate", instance, "count", len(bf))
		} else {
			log.Debug("no base files found", "candidate", instance)
		}

		appRepo := instance

		// Compute the ordered list of envs this candidate was actually found in,
		// preserving the configured order so steps are always sequenced correctly.
		var candidateEnvs []string
		for _, env := range d.Envs {
			if _, ok := meta[instance].envs[env]; ok {
				candidateEnvs = append(candidateEnvs, env)
			}
		}

		// Build per-candidate steps if a builder is configured.
		var candidateSteps *[]api.StepDefinition
		if d.StepBuilder != nil {
			built := d.StepBuilder(candidateEnvs)
			candidateSteps = &built
		}

		candidates = append(candidates, api.Candidate{
			Id:     instance,
			Kind:   "application",
			Status: api.CandidateStatusNotStarted,
			Metadata: &map[string]string{
				"repoName": appRepo,
			},
			Files: &fileGroups,
			Steps: candidateSteps,
		})
	}

	return candidates, nil
}
