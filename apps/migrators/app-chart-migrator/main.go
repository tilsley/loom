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
	workerURL := envOr("WORKER_URL", "http://localhost:8082")
	port := envOr("PORT", "8082")

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

	// --- Redis (pending callback store) ---

	redisAddr := envOr("REDIS_ADDR", "localhost:6379")

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

	store := pending.NewStore(redisAddr, log)
	loomClient := loom.NewClient(loomURL, log)
	dispatch := handler.NewDispatch(ghAdapter, store, loomClient, log, stepCfg)
	webhook := handler.NewWebhook(store, loomClient, log)
	dryRunRunner := &dryrun.Runner{RealClient: ghAdapter, StepCfg: stepCfg}
	dryRunHandler := handler.NewDryRun(dryRunRunner, log)

	r := gin.Default()

	// Steps are dispatched here directly from the server (no Dapr).
	r.POST("/dispatch-step", dispatch.Handle)

	// GitHub webhook — called when a PR is merged
	r.POST("/webhooks/github", webhook.Handle)

	// Dry run — simulate all steps, return file diffs
	r.POST("/dry-run", dryRunHandler.Handle)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	ctx := context.Background()

	// Announce migration on startup, then discover candidates.
	discoverer := &discovery.AppChartDiscoverer{
		Reader:      ghAdapter,
		GitopsOwner: gitopsOwner,
		GitopsRepo:  gitopsRepoName,
		Envs:        envs,
		StepBuilder: buildStepDefs,
		Log:         log,
	}
	discoveryRunner := &discovery.Runner{
		MigrationID: "app-chart-migration",
		Discoverer:  discoverer,
		ServerURL:   loomURL,
		Log:         log,
	}
	go func() {
		if !announceOnStartup(log, loomURL, workerURL, gitopsOwner, gitopsRepoName, envs) {
			return
		}
		discoveryRunner.Run(ctx)
	}()

	log.Info("worker starting", "port", port, "gitopsRepo", gitopsRepo, "envs", envs, "migratorUrl", workerURL)
	if err := r.Run(":" + port); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// buildStepDefs returns the ordered step definitions for the given environments.
// The returned list is the full step sequence for a candidate that exists in
// exactly those environments. buildAnnouncement calls this with all envs to
// produce the migration-level template; the discoverer calls it per-candidate
// with only the envs the candidate was actually found in.
func buildStepDefs(envs []string) []api.StepDefinition {
	defs := make([]api.StepDefinition, 0, 3+4*len(envs)+2)
	defs = append(defs,
		api.StepDefinition{
			Name:        "disable-base-resource-prune",
			Description: strPtr("Add Prune=false sync option to base/common non-Argo resources"),
			MigratorApp: "app-chart-migrator",
			Type:        strPtr("disable-base-resource-prune"),
		},
		api.StepDefinition{
			Name:        "generate-app-chart",
			Description: strPtr("Generate app-specific Helm chart with per-env values"),
			MigratorApp: "app-chart-migrator",
			Type:        strPtr("generate-app-chart"),
		},
		api.StepDefinition{
			Name:        "verify-ecr-publish",
			Description: strPtr("Confirm app chart has been published to ECR"),
			MigratorApp: "app-chart-migrator",
			Type:        strPtr("manual-review"),
			Config: &map[string]string{
				"instructions": "1. Open ECR in the AWS console\n2. Find the repository for this application\n3. Confirm the new chart version has been published successfully\n4. Verify the image digest and tags look correct",
			},
		},
	)

	for _, env := range envs {
		defs = append(defs,
			api.StepDefinition{
				Name:        "disable-sync-prune-" + env,
				Description: strPtr("Disable sync pruning for " + env),
				MigratorApp: "app-chart-migrator",
				Type:        strPtr("disable-sync-prune"),
				Config:      &map[string]string{"env": env},
			},
			api.StepDefinition{
				Name:        "swap-chart-" + env,
				Description: strPtr("Swap to OCI app chart for " + env),
				MigratorApp: "app-chart-migrator",
				Type:        strPtr("swap-chart"),
				Config:      &map[string]string{"env": env},
			},
			api.StepDefinition{
				Name:        "review-swap-chart-" + env,
				Description: strPtr("Manual review of ArgoCD after chart swap for " + env),
				MigratorApp: "app-chart-migrator",
				Type:        strPtr("manual-review"),
				Config: &map[string]string{
					"instructions": "1. Open the ArgoCD UI\n2. Find the application in the " + env + " environment\n3. Verify app health is Healthy\n4. Verify sync status is Synced\n5. Check no resources are OutOfSync or orphaned\n6. Confirm pods are running with expected image",
				},
			},
			api.StepDefinition{
				Name:        "enable-sync-prune-" + env,
				Description: strPtr("Re-enable sync pruning for " + env),
				MigratorApp: "app-chart-migrator",
				Type:        strPtr("enable-sync-prune"),
				Config:      &map[string]string{"env": env},
			},
		)
	}

	defs = append(defs,
		api.StepDefinition{
			Name:        "cleanup-common",
			Description: strPtr("Remove old helm values from base application"),
			MigratorApp: "app-chart-migrator",
			Type:        strPtr("cleanup-common"),
		},
		api.StepDefinition{
			Name:        "update-deploy-workflow",
			Description: strPtr("Update CI workflow for app chart deployment"),
			MigratorApp: "app-chart-migrator",
			Type:        strPtr("update-deploy-workflow"),
		},
	)
	return defs
}

func buildAnnouncement(workerURL, gitopsOwner, gitopsRepoName string, envs []string) api.MigrationAnnouncement {
	desc := "Migrate from generic Helm chart with per-env helm.parameters to app-specific OCI wrapper charts"

	return api.MigrationAnnouncement{
		Id:             "app-chart-migration",
		Name:           "App Chart Migration",
		Description:    desc,
		RequiredInputs: &[]api.InputDefinition{{Name: "repoName", Label: "Repository", Description: strPtr("Pre-filled from discovery — verify before continuing")}},
		Candidates:     []api.Candidate{},
		Steps:          buildStepDefs(envs),
		MigratorUrl:    workerURL,
	}
}

func announceOnStartup(log *slog.Logger, loomURL, workerURL, gitopsOwner, gitopsRepoName string, envs []string) bool {
	// Small pause to let the server start accepting connections.
	time.Sleep(2 * time.Second)

	announcement := buildAnnouncement(workerURL, gitopsOwner, gitopsRepoName, envs)

	body, err := json.Marshal(announcement)
	if err != nil {
		log.Error("failed to marshal announcement", "error", err)
		return false
	}

	url := loomURL + "/registry/announce"

	// Retry up to 10 times (server may not be ready yet).
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
				return true
			}
			log.Warn("announce returned non-2xx", "status", resp.StatusCode, "attempt", i+1)
		} else {
			log.Warn("announce failed", "error", err, "attempt", i+1)
		}
		time.Sleep(2 * time.Second)
	}

	log.Error("failed to announce migration after retries")
	return false
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
