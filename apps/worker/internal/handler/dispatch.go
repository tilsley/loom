package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/apps/worker/internal/github"
	"github.com/tilsley/loom/apps/worker/internal/pending"
	"github.com/tilsley/loom/apps/worker/internal/steps"
	"github.com/tilsley/loom/pkg/api"
)

// Dispatch handles incoming DispatchStepRequest messages from Dapr pub/sub.
type Dispatch struct {
	gh      *github.Client
	pending *pending.Store
	loomURL string
	log     *slog.Logger
	stepCfg *steps.Config
}

// NewDispatch creates a Dispatch handler.
func NewDispatch(
	gh *github.Client,
	store *pending.Store,
	loomURL string,
	log *slog.Logger,
	stepCfg *steps.Config,
) *Dispatch {
	return &Dispatch{gh: gh, pending: store, loomURL: loomURL, log: log, stepCfg: stepCfg}
}

// Handle processes a CloudEvent-wrapped DispatchStepRequest.
func (d *Dispatch) Handle(c *gin.Context) {
	var envelope struct {
		Data api.DispatchStepRequest `json:"data"`
	}
	if err := c.ShouldBindJSON(&envelope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req := envelope.Data

	d.log.Info("received dispatch", "step", req.StepName, "target", req.Target.Repo, "callbackId", req.CallbackId)

	// Notify Loom that work has started (console shows spinner).
	d.sendProgress(c.Request.Context(), req, map[string]string{"phase": "in_progress"})

	// Route to step handler if config["type"] is present.
	result, handled, err := d.routeToHandler(c.Request.Context(), req)
	if err != nil {
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

	// Fallback to generic behavior if no handler matched
	if owner == "" {
		parts := strings.SplitN(req.Target.Repo, "/", 2)
		if len(parts) != 2 {
			d.log.Error("invalid target format (expected owner/repo)", "target", req.Target.Repo)
			c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
			return
		}
		owner, repo = parts[0], parts[1]
		title = fmt.Sprintf("[%s] %s — %s", req.MigrationId, req.StepName, req.Target.Repo)
		body = fmt.Sprintf("Automated migration step `%s` for repo `%s`.", req.StepName, req.Target.Repo)
		branch = fmt.Sprintf("loom/%s/%s", req.MigrationId, req.StepName)
	}

	// Create PR on GitHub
	pr, err := d.gh.CreatePR(c.Request.Context(), owner, repo, github.CreatePRRequest{
		Title: title,
		Body:  body,
		Head:  branch,
		Base:  "main",
		Files: files,
	})
	if err != nil {
		d.log.Error("failed to create PR", "error", err, "target", req.Target.Repo)
		c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
		return
	}

	d.log.Info("PR created", "target", req.Target.Repo, "repo", owner+"/"+repo, "pr", pr.HTMLURL)

	// Store pending callback — the webhook handler will complete the step
	// when the PR is merged.
	key := fmt.Sprintf("%s/%s#%d", owner, repo, pr.Number)
	d.pending.Add(key, pending.Callback{
		CallbackID: req.CallbackId,
		StepName:   req.StepName,
		Target:     req.Target,
		PRURL:      pr.HTMLURL,
	})

	// Notify Loom that a PR is open and awaiting review.
	d.sendProgress(c.Request.Context(), req, map[string]string{
		"phase": "open",
		"prUrl": pr.HTMLURL,
	})

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
}

// routeToHandler looks up the registered step handler for req.Config["type"] and executes it.
// Returns handled=false when no handler is registered for the step type.
func (d *Dispatch) routeToHandler(ctx context.Context, req api.DispatchStepRequest) (*steps.Result, bool, error) {
	if req.Config == nil {
		return nil, false, nil
	}
	stepType, ok := (*req.Config)["type"]
	if !ok {
		return nil, false, nil
	}
	h, found := steps.Lookup(stepType)
	if !found {
		return nil, false, nil
	}
	d.log.Info("executing step handler", "type", stepType, "target", req.Target.Repo)
	result, err := h.Execute(ctx, d.gh, d.stepCfg, req)
	if err != nil {
		d.log.Error("step handler failed", "error", err, "type", stepType)
		return nil, false, err
	}
	return result, true, nil
}

// sendProgress sends an intermediate progress update to Loom's pr-opened endpoint.
func (d *Dispatch) sendProgress(ctx context.Context, req api.DispatchStepRequest, meta map[string]string) {
	event := api.StepCompletedEvent{
		StepName: req.StepName,
		Target:   req.Target,
		Success:  true,
		Metadata: &meta,
	}

	body, err := json.Marshal(event)
	if err != nil {
		d.log.Error("failed to marshal progress", "error", err)
		return
	}

	url := fmt.Sprintf("%s/event/%s/pr-opened", d.loomURL, req.CallbackId)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		d.log.Error("failed to create progress request", "error", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		d.log.Error("failed to send progress", "error", err, "url", url)
		return
	}
	defer func() { //nolint:errcheck // response body close errors are non-actionable after reading
		_ = resp.Body.Close()
	}()

	d.log.Info("progress sent", "phase", meta["phase"], "step", req.StepName, "target", req.Target.Repo)
}
