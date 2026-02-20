package main

import (
	"log"
	"os"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/apps/server/internal/migrations/adapters"
	"github.com/tilsley/loom/apps/server/internal/migrations/execution"
	"github.com/tilsley/loom/apps/server/internal/platform/logger"
	temporalplatform "github.com/tilsley/loom/apps/server/internal/platform/temporal"
)

func main() {
	slog := logger.New()

	// --- Platform: Temporal ---

	hostPort := os.Getenv("TEMPORAL_HOSTPORT")
	if hostPort == "" {
		hostPort = "localhost:7233"
	}

	tc, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		slog.Error("temporal client init failed", "error", err)
		os.Exit(1)
	}
	defer tc.Close()

	engine := temporalplatform.NewEngine(tc)

	// --- Platform: Dapr (pub/sub + state store) ---

	daprClient, err := dapr.NewClient()
	if err != nil {
		tc.Close() // explicitly close before os.Exit so defer doesn't get skipped
		slog.Error("dapr client init failed", "error", err)
		os.Exit(1) //nolint:gocritic // tc.Close() called explicitly above
	}
	defer daprClient.Close()

	// --- Adapters ---

	bus := adapters.NewDaprBus(daprClient, "pubsub", "migration-steps")
	store := adapters.NewDaprMigrationStore(daprClient)

	// --- Temporal Worker ---

	activities := execution.NewActivities(bus, store, slog)

	w := worker.New(tc, temporalplatform.TaskQueue(), worker.Options{})
	w.RegisterWorkflowWithOptions(execution.MigrationOrchestrator, workflow.RegisterOptions{
		Name: "MigrationOrchestrator",
	})
	w.RegisterActivity(activities)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("temporal worker failed: %v", err)
		}
	}()
	slog.Info("temporal worker started", "taskQueue", temporalplatform.TaskQueue())

	// --- Service + HTTP ---

	svc := migrations.NewService(engine, store)

	router := gin.Default()
	adapters.RegisterRoutes(router, svc, slog)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("starting loom", "port", port)
	if err := router.Run(":" + port); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
