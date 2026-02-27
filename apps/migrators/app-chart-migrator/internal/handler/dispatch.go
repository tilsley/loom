package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/gitrepo"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/platform/loom"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/platform/pending"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/steps"
	"github.com/tilsley/loom/pkg/api"
)

// Dispatch handles incoming DispatchStepRequest messages from Dapr pub/sub.
type Dispatch struct {
	gr      gitrepo.Client
	pending *pending.Store
	loom    *loom.Client
	log     *slog.Logger
	stepCfg *steps.Config
}

// NewDispatch creates a Dispatch handler.
func NewDispatch(
	gr gitrepo.Client,
	store *pending.Store,
	loomClient *loom.Client,
	log *slog.Logger,
	stepCfg *steps.Config,
) *Dispatch {
	return &Dispatch{gr: gr, pending: store, loom: loomClient, log: log, stepCfg: stepCfg}
}

// Handle processes a DispatchStepRequest posted directly by the server.
func (d *Dispatch) Handle(c *gin.Context) {
	var req api.DispatchStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	d.log.Info("received dispatch", "step", req.StepName, "target", req.Candidate.Id, "callbackId", req.CallbackId)

	// Route to the registered step handler (by req.Type).
	result, handled, err := d.routeToHandler(c.Request.Context(), req)
	if err != nil {
		// Signal the workflow that this step failed so it can surface in the UI.
		_ = d.loom.SendCallback(c.Request.Context(), req.CallbackId, api.StepStatusEvent{
			StepName:    req.StepName,
			CandidateId: req.Candidate.Id,
			Status:      api.StepStatusEventStatusFailed,
		})
		c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
		return
	}

	// Handler acknowledged the step but requires no PR (e.g. manual-review).
	// Signal "pending" with instructions so the UI shows the Approve/Reject buttons.
	if handled && result.Owner == "" {
		d.loom.SendUpdate(c.Request.Context(), req.CallbackId, req.StepName, req.Candidate.Id, map[string]string{"instructions": result.Instructions})
		c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
		return
	}

	var owner, repo, title, body, branch string
	var files map[string]string
	if handled {
		owner = result.Owner
		repo = result.Repo
		title = result.Title
		body = result.Body
		branch = result.Branch
		files = result.Files
	}

	// Fallback to generic behavior if no handler matched.
	if owner == "" {
		repoName := req.Candidate.Id
		if req.Candidate.Metadata != nil {
			if rn, ok := (*req.Candidate.Metadata)["repoName"]; ok && rn != "" {
				repoName = rn
			}
		}
		parts := strings.SplitN(repoName, "/", 2)
		if len(parts) != 2 {
			d.log.Error("invalid target format (expected owner/repo in repoName metadata)", "candidate", req.Candidate.Id)
			c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
			return
		}
		owner, repo = parts[0], parts[1]
		title = fmt.Sprintf("[%s] %s — %s", req.MigrationId, req.StepName, req.Candidate.Id)
		body = fmt.Sprintf("Automated migration step `%s` for candidate `%s`.", req.StepName, req.Candidate.Id)
		branch = fmt.Sprintf("loom/%s/%s--%s", req.MigrationId, req.StepName, req.Candidate.Id)
	}

	// Create PR on GitHub.
	pr, err := d.gr.CreatePR(c.Request.Context(), owner, repo, gitrepo.CreatePRRequest{
		Title: title,
		Body:  body,
		Head:  branch,
		Base:  "main",
		Files: files,
	})
	if err != nil {
		d.log.Error("failed to create PR", "error", err, "target", req.Candidate.Id)
		_ = d.loom.SendCallback(c.Request.Context(), req.CallbackId, api.StepStatusEvent{
			StepName:    req.StepName,
			CandidateId: req.Candidate.Id,
			Status:      api.StepStatusEventStatusFailed,
		})
		c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
		return
	}

	d.log.Info("PR created", "target", req.Candidate.Id, "repo", owner+"/"+repo, "pr", pr.HTMLURL)

	// Store pending callback — the webhook handler will complete the step
	// when the PR is merged.
	key := fmt.Sprintf("%s/%s#%d", owner, repo, pr.Number)
	d.pending.Add(key, pending.Callback{
		CallbackID:  req.CallbackId,
		StepName:    req.StepName,
		CandidateId: req.Candidate.Id,
		PRURL:       pr.HTMLURL,
	})

	// Notify the workflow that a PR is open so the UI can show the link
	// and the "Mark as merged" button while waiting for the webhook.
	d.loom.SendUpdate(c.Request.Context(), req.CallbackId, req.StepName, req.Candidate.Id, map[string]string{"prUrl": pr.HTMLURL})

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
}

// routeToHandler looks up the registered step handler for req.Type and executes it.
// Returns handled=false when no handler is registered for the step type.
func (d *Dispatch) routeToHandler(ctx context.Context, req api.DispatchStepRequest) (*steps.Result, bool, error) {
	if req.Type == nil {
		return nil, false, nil
	}
	h, found := steps.Lookup(*req.Type)
	if !found {
		return nil, false, nil
	}
	d.log.Info("executing step handler", "type", *req.Type, "target", req.Candidate.Id)
	result, err := h.Execute(ctx, d.gr, d.stepCfg, req)
	if err != nil {
		d.log.Error("step handler failed", "error", err, "type", *req.Type)
		return nil, false, err
	}
	return result, true, nil
}
