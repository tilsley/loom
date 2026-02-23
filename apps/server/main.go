package main

import (
	"context"
	"log"
	"os"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	otelcontrib "go.temporal.io/sdk/contrib/opentelemetry"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/apps/server/internal/migrations/adapters"
	"github.com/tilsley/loom/apps/server/internal/migrations/execution"
	"github.com/tilsley/loom/apps/server/internal/platform/logger"
	temporalplatform "github.com/tilsley/loom/apps/server/internal/platform/temporal"
	"github.com/tilsley/loom/apps/server/internal/platform/telemetry"
	"github.com/tilsley/loom/apps/server/internal/platform/validation"
	"github.com/tilsley/loom/schemas"
)

func main() {
	slog := logger.New()

	// --- Observability ---

	// Default the service name before any OTel init.
	if os.Getenv("OTEL_SERVICE_NAME") == "" {
		os.Setenv("OTEL_SERVICE_NAME", "loom-server") //nolint:errcheck
	}

	otelEnabled := os.Getenv("OTEL_ENABLED") == "true"
	ctx := context.Background()
	tel, err := telemetry.New(ctx, otelEnabled)
	if err != nil {
		slog.Error("telemetry init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tel.Shutdown(shutdownCtx); err != nil {
			slog.Error("telemetry shutdown failed", "error", err)
		}
	}()

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
	dryRunner := adapters.NewDaprDryRunAdapter(daprClient, "app-chart-migrator")

	// --- Temporal Worker ---

	activities := execution.NewActivities(bus, store, slog)

	workerOpts := worker.Options{}
	if otelEnabled {
		tracingInterceptor, err := otelcontrib.NewTracingInterceptor(otelcontrib.TracerOptions{})
		if err != nil {
			slog.Error("temporal tracing interceptor init failed", "error", err)
			os.Exit(1)
		}
		workerOpts.Interceptors = []interceptor.WorkerInterceptor{tracingInterceptor}
	}

	w := worker.New(tc, temporalplatform.TaskQueue(), workerOpts)
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

	svc := migrations.NewService(engine, store, dryRunner)

	router := gin.New()

	validator, err := validation.New(schemas.OpenAPISpec)
	if err != nil {
		slog.Error("openapi validation middleware init failed", "error", err)
		os.Exit(1)
	}

	router.Use(gin.Recovery(), otelgin.Middleware("loom-server"), validator)
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
