package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/gin-gonic/gin"

	"github.com/tilsley/loom/apps/worker/internal/github"
	"github.com/tilsley/loom/apps/worker/internal/handler"
	"github.com/tilsley/loom/apps/worker/internal/pending"
	"github.com/tilsley/loom/apps/worker/internal/steps"
	"github.com/tilsley/loom/pkg/api"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	githubURL := envOr("GITHUB_API_URL", "http://localhost:9090")
	loomURL := envOr("LOOM_URL", "http://localhost:8080")
	daprPort := envOr("DAPR_HTTP_PORT", "3501")
	port := envOr("PORT", "3001")

	// Parse gitops repo from env
	gitopsRepo := envOr("GITOPS_REPO", "acme/gitops")
	gitopsParts := strings.SplitN(gitopsRepo, "/", 2)
	gitopsOwner, gitopsRepoName := gitopsParts[0], gitopsParts[1]

	// Parse environments
	envsStr := envOr("ENVS", "dev,staging,prod")
	envs := strings.Split(envsStr, ",")

	stepCfg := &steps.Config{
		GitopsOwner: gitopsOwner,
		GitopsRepo:  gitopsRepoName,
		Envs:        envs,
	}

	// --- Platform: Dapr ---

	daprClient, err := dapr.NewClient()
	if err != nil {
		log.Error("dapr client init failed", "error", err)
		os.Exit(1)
	}
	defer daprClient.Close()

	gh := github.NewClient(githubURL)
	store := pending.NewStore(daprClient, log)
	dispatch := handler.NewDispatch(gh, store, loomURL, log, stepCfg)
	webhook := handler.NewWebhook(store, loomURL, log)

	r := gin.Default()

	// Dapr subscription discovery
	r.GET("/dapr/subscribe", func(c *gin.Context) {
		c.JSON(http.StatusOK, []gin.H{
			{
				"pubsubname": "pubsub",
				"topic":      "migration-steps",
				"route":      "/steps/dispatch",
			},
		})
	})

	// Dapr delivers dispatched steps here
	r.POST("/steps/dispatch", dispatch.Handle)

	// GitHub webhook — called when a PR is merged
	r.POST("/webhooks/github", webhook.Handle)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Announce migration on startup (after sidecar is ready)
	go announceOnStartup(log, daprPort, gitopsOwner, gitopsRepoName, envs)

	log.Info("worker starting", "port", port, "gitopsRepo", gitopsRepo, "envs", envs)
	if err := r.Run(":" + port); err != nil {
		daprClient.Close() // explicitly close before os.Exit so defer doesn't get skipped
		log.Error("server failed", "error", err)
		os.Exit(1) //nolint:gocritic // daprClient.Close() called explicitly above
	}
}

func buildAnnouncement(gitopsOwner, gitopsRepoName string, envs []string) api.MigrationAnnouncement {
	desc := "Migrate from generic Helm chart with per-env helm.parameters to app-specific OCI wrapper charts"

	// GitHub URL bases. {appName} and {repo} are template variables substituted
	// by the console when displaying a specific target's run.
	gitopsBase := fmt.Sprintf("https://github.com/%s/%s/blob/main", gitopsOwner, gitopsRepoName)
	appRepoBase := "https://github.com/{repo}/blob/main"

	// generate-app-chart creates several files in the app repo
	generateFiles := make([]string, 0, 3+len(envs))
	generateFiles = append(generateFiles,
		appRepoBase+"/charts/{appName}/Chart.yaml",
		appRepoBase+"/charts/{appName}/values.yaml",
		appRepoBase+"/.github/workflows/publish-chart.yaml",
	)
	for _, env := range envs {
		generateFiles = append(generateFiles, appRepoBase+fmt.Sprintf("/charts/{appName}/values-%s.yaml", env))
	}

	stepDefs := make([]api.StepDefinition, 0, 2+4*len(envs)+2)
	stepDefs = append(stepDefs,
		api.StepDefinition{
			Name:        "disable-resource-prune",
			Description: strPtr("Add Prune=false sync option to non-Argo resources"),
			WorkerApp:   "migration-worker",
			Config:      &map[string]string{"type": "disable-resource-prune"},
			Files:       &[]string{gitopsBase + "/apps/{appName}/base/service-monitor.yaml"},
		},
		api.StepDefinition{
			Name:        "generate-app-chart",
			Description: strPtr("Generate app-specific Helm chart with per-env values"),
			WorkerApp:   "migration-worker",
			Config:      &map[string]string{"type": "generate-app-chart"},
			Files:       &generateFiles,
		},
	)

	// Per-env steps: disable-sync-prune → swap-chart → review → enable-sync-prune
	for _, env := range envs {
		overlayFile := gitopsBase + fmt.Sprintf("/apps/{appName}/overlays/%s/application.yaml", env)
		stepDefs = append(stepDefs,
			api.StepDefinition{
				Name:        "disable-sync-prune-" + env,
				Description: strPtr("Disable sync pruning for " + env),
				WorkerApp:   "migration-worker",
				Config:      &map[string]string{"type": "disable-sync-prune", "env": env},
				Files:       &[]string{overlayFile},
			},
			api.StepDefinition{
				Name:        "swap-chart-" + env,
				Description: strPtr("Swap to OCI app chart for " + env),
				WorkerApp:   "migration-worker",
				Config:      &map[string]string{"type": "swap-chart", "env": env},
				Files:       &[]string{overlayFile},
			},
			api.StepDefinition{
				Name:        "review-swap-chart-" + env,
				Description: strPtr("Manual review of ArgoCD after chart swap for " + env),
				WorkerApp:   "loom",
				Config: &map[string]string{
					"type":         "manual-review",
					"instructions": "1. Open the ArgoCD UI\n2. Find the application in the " + env + " environment\n3. Verify app health is Healthy\n4. Verify sync status is Synced\n5. Check no resources are OutOfSync or orphaned\n6. Confirm pods are running with expected image",
				},
			},
			api.StepDefinition{
				Name:        "enable-sync-prune-" + env,
				Description: strPtr("Re-enable sync pruning for " + env),
				WorkerApp:   "migration-worker",
				Config:      &map[string]string{"type": "enable-sync-prune", "env": env},
				Files:       &[]string{overlayFile},
			},
		)
	}

	stepDefs = append(stepDefs,
		api.StepDefinition{
			Name:        "cleanup-common",
			Description: strPtr("Remove old helm values from base application"),
			WorkerApp:   "migration-worker",
			Config:      &map[string]string{"type": "cleanup-common"},
			Files:       &[]string{gitopsBase + "/apps/{appName}/base/application.yaml"},
		},
		api.StepDefinition{
			Name:        "update-deploy-workflow",
			Description: strPtr("Update CI workflow for app chart deployment"),
			WorkerApp:   "migration-worker",
			Config:      &map[string]string{"type": "update-deploy-workflow"},
			Files:       &[]string{appRepoBase + "/.github/workflows/ci.yaml"},
		},
	)

	return api.MigrationAnnouncement{
		Id:          "app-chart-migration",
		Name:        "App Chart Migration",
		Description: &desc,
		Targets: []api.Target{
			{Repo: "acme/billing-api", Metadata: &map[string]string{"appName": "billing-api"}},
			{Repo: "acme/user-service", Metadata: &map[string]string{"appName": "user-service"}},
		},
		Steps: stepDefs,
	}
}

func announceOnStartup(log *slog.Logger, daprPort, gitopsOwner, gitopsRepoName string, envs []string) {
	// Wait for Dapr sidecar readiness
	time.Sleep(2 * time.Second)

	announcement := buildAnnouncement(gitopsOwner, gitopsRepoName, envs)

	body, err := json.Marshal(announcement)
	if err != nil {
		log.Error("failed to marshal announcement", "error", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%s/v1.0/publish/pubsub/migration-registry", daprPort)

	// Retry up to 10 times (sidecar may not be ready yet)
	for i := range 10 {
		httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			log.Warn("announce: failed to build request", "error", err)
			break
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err == nil {
			_ = resp.Body.Close() //nolint:errcheck // response body close errors are non-actionable
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				log.Info("migration announced", "id", announcement.Id, "steps", len(announcement.Steps))
				return
			}
			log.Warn("announce returned non-2xx", "status", resp.StatusCode, "attempt", i+1)
		} else {
			log.Warn("announce failed", "error", err, "attempt", i+1)
		}
		time.Sleep(2 * time.Second)
	}

	log.Error("failed to announce migration after retries")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func strPtr(s string) *string {
	return &s
}
