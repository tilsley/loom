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

	githubadapter "github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/adapters/github"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/discovery"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/dryrun"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/handler"
	platformgithub "github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/platform/github"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/platform/loom"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/platform/pending"
	"github.com/tilsley/loom/apps/migrators/app-chart-migrator/internal/steps"
	"github.com/tilsley/loom/pkg/api"
	"github.com/tilsley/loom/pkg/logging"
)

func main() {
	log := logging.New()

	githubURL := envOr("GITHUB_API_URL", "http://localhost:9090")
	loomURL := envOr("LOOM_URL", "http://localhost:8080")
	daprPort := envOr("DAPR_HTTP_PORT", "3501")
	port := envOr("PORT", "3001")

	// Parse gitops repo from env
	gitopsRepo := envOr("GITOPS_REPO", "tilsley/gitops")
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

	// --- GitHub adapter ---
	// Token auth (local dev / CI): set GITHUB_TOKEN.
	// App auth (deployed):         set GITHUB_APP_ID, GITHUB_APP_INSTALLATION_ID,
	//                              and GITHUB_APP_PRIVATE_KEY_PATH.
	var ghAdapter *githubadapter.Adapter
	appID := envOr("GITHUB_APP_ID", "")
	appInstallID := envOr("GITHUB_APP_INSTALLATION_ID", "")
	appKeyPath := envOr("GITHUB_APP_PRIVATE_KEY_PATH", "")

	if appID != "" && appInstallID != "" && appKeyPath != "" {
		var parsedAppID, parsedInstallID int64
		if _, err := fmt.Sscanf(appID, "%d", &parsedAppID); err != nil {
			log.Error("invalid GITHUB_APP_ID", "error", err)
			os.Exit(1)
		}
		if _, err := fmt.Sscanf(appInstallID, "%d", &parsedInstallID); err != nil {
			log.Error("invalid GITHUB_APP_INSTALLATION_ID", "error", err)
			os.Exit(1)
		}
		ghClient, err := platformgithub.NewAppClient(parsedAppID, parsedInstallID, appKeyPath, githubURL)
		if err != nil {
			log.Error("github app auth failed", "error", err)
			os.Exit(1)
		}
		ghAdapter = githubadapter.New(ghClient)
		log.Info("github: using app auth", "appID", parsedAppID, "installationID", parsedInstallID)
	} else {
		ghClient := platformgithub.NewTokenClient(os.Getenv("GITHUB_TOKEN"), githubURL)
		ghAdapter = githubadapter.New(ghClient)
		log.Info("github: using token auth", "url", githubURL)
	}

	store := pending.NewStore(daprClient, log)
	loomClient := loom.NewClient(loomURL, log)
	dispatch := handler.NewDispatch(ghAdapter, store, loomClient, log, stepCfg)
	webhook := handler.NewWebhook(store, loomClient, log)
	dryRunRunner := &dryrun.Runner{RealClient: ghAdapter, StepCfg: stepCfg}
	dryRunHandler := handler.NewDryRun(dryRunRunner, log)

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

	// Dry run — simulate all steps, return file diffs
	r.POST("/dry-run", dryRunHandler.Handle)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	ctx := context.Background()

	// Announce migration on startup (after sidecar is ready)
	go announceOnStartup(log, daprPort, gitopsOwner, gitopsRepoName, envs)

	// Run candidate discovery once on startup (after announce completes)
	discoverer := &discovery.AppChartDiscoverer{
		Reader:      ghAdapter,
		GitopsOwner: gitopsOwner,
		GitopsRepo:  gitopsRepoName,
		Log:         log,
	}
	discoveryRunner := &discovery.Runner{
		MigrationID: "app-chart-migration",
		Discoverer:  discoverer,
		ServerURL:   loomURL,
		Log:         log,
	}
	go discoveryRunner.Run(ctx)

	log.Info("worker starting", "port", port, "gitopsRepo", gitopsRepo, "envs", envs)
	if err := r.Run(":" + port); err != nil {
		daprClient.Close() // explicitly close before os.Exit so defer doesn't get skipped
		log.Error("server failed", "error", err)
		os.Exit(1) //nolint:gocritic // daprClient.Close() called explicitly above
	}
}

func buildAnnouncement(gitopsOwner, gitopsRepoName string, envs []string) api.MigrationAnnouncement {
	desc := "Migrate from generic Helm chart with per-env helm.parameters to app-specific OCI wrapper charts"

	stepDefs := make([]api.StepDefinition, 0, 2+4*len(envs)+2)
	stepDefs = append(stepDefs,
		api.StepDefinition{
			Name:        "disable-base-resource-prune",
			Description: strPtr("Add Prune=false sync option to base/common non-Argo resources"),
			WorkerApp:   "app-chart-migrator",
			Config:      &map[string]string{"type": "disable-base-resource-prune"},
		},
		api.StepDefinition{
			Name:        "generate-app-chart",
			Description: strPtr("Generate app-specific Helm chart with per-env values"),
			WorkerApp:   "app-chart-migrator",
			Config:      &map[string]string{"type": "generate-app-chart"},
		},
	)

	// Per-env steps: disable-sync-prune → swap-chart → review → enable-sync-prune
	for _, env := range envs {
		stepDefs = append(stepDefs,
			api.StepDefinition{
				Name:        "disable-sync-prune-" + env,
				Description: strPtr("Disable sync pruning for " + env),
				WorkerApp:   "app-chart-migrator",
				Config:      &map[string]string{"type": "disable-sync-prune", "env": env},
			},
			api.StepDefinition{
				Name:        "swap-chart-" + env,
				Description: strPtr("Swap to OCI app chart for " + env),
				WorkerApp:   "app-chart-migrator",
				Config:      &map[string]string{"type": "swap-chart", "env": env},
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
				WorkerApp:   "app-chart-migrator",
				Config:      &map[string]string{"type": "enable-sync-prune", "env": env},
			},
		)
	}

	stepDefs = append(stepDefs,
		api.StepDefinition{
			Name:        "cleanup-common",
			Description: strPtr("Remove old helm values from base application"),
			WorkerApp:   "app-chart-migrator",
			Config:      &map[string]string{"type": "cleanup-common"},
		},
		api.StepDefinition{
			Name:        "update-deploy-workflow",
			Description: strPtr("Update CI workflow for app chart deployment"),
			WorkerApp:   "app-chart-migrator",
			Config:      &map[string]string{"type": "update-deploy-workflow"},
		},
	)

	return api.MigrationAnnouncement{
		Id:             "app-chart-migration",
		Name:           "App Chart Migration",
		Description:    &desc,
		RequiredInputs: &[]string{"repoName"},
		Candidates:     []api.Candidate{},
		Steps:          stepDefs,
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
