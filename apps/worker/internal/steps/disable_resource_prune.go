package steps

import (
	"context"
	"fmt"

	"github.com/tilsley/loom/apps/worker/internal/gitrepo"
	"github.com/tilsley/loom/apps/worker/internal/yamlutil"
	"github.com/tilsley/loom/pkg/api"
)

// DisableResourcePrune adds Prune=false sync option to non-Application resources
// in the "base" file group so ArgoCD won't delete them during the chart swap.
//
// The files to patch are discovered upfront and stored in the candidate's "base"
// file group (resources from common/ or common-non-prod/ directories in the
// gitops repo). This step fetches each file, annotates it, and includes it in a PR.
type DisableResourcePrune struct{}

// Execute implements Handler.
func (h *DisableResourcePrune) Execute(
	ctx context.Context,
	gr gitrepo.Client,
	cfg *Config,
	req api.DispatchStepRequest,
) (*Result, error) {
	app := appName(req.Candidate)

	// Find the "base" file group — non-Application resources in common/base dirs.
	var baseGroup *api.FileGroup
	if req.Candidate.Files != nil {
		for _, group := range *req.Candidate.Files {
			if group.Name == "base" {
				g := group
				baseGroup = &g
				break
			}
		}
	}

	if baseGroup == nil || len(baseGroup.Files) == 0 {
		// No base resources discovered for this candidate — nothing to do.
		return &Result{
			Owner:  cfg.GitopsOwner,
			Repo:   cfg.GitopsRepo,
			Title:  fmt.Sprintf("[%s] Disable resource pruning for %s", req.MigrationId, app),
			Body:   fmt.Sprintf("No base resources found for `%s` — nothing to patch.", app),
			Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
			Files:  map[string]string{},
		}, nil
	}

	prFiles := make(map[string]string)
	for _, fileRef := range baseGroup.Files {
		fc, err := gr.GetContents(ctx, cfg.GitopsOwner, cfg.GitopsRepo, fileRef.Path)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", fileRef.Path, err)
		}

		root, err := yamlutil.ParseNode(fc.Content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", fileRef.Path, err)
		}

		annotations := yamlutil.EnsureMappingNode(root, "metadata", "annotations")
		yamlutil.SetScalar(annotations, "argocd.argoproj.io/sync-options", "Prune=false")

		out, err := yamlutil.MarshalNode(root)
		if err != nil {
			return nil, fmt.Errorf("marshal %s: %w", fileRef.Path, err)
		}
		prFiles[fileRef.Path] = out
	}

	return &Result{
		Owner: cfg.GitopsOwner,
		Repo:  cfg.GitopsRepo,
		Title: fmt.Sprintf("[%s] Disable resource pruning for %s", req.MigrationId, app),
		Body: fmt.Sprintf(
			"Add `Prune=false` sync option to base resources for `%s`.\n\nThis prevents ArgoCD from deleting them during the chart swap.",
			app,
		),
		Branch: fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName),
		Files:  prFiles,
	}, nil
}
