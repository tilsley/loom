package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.temporal.io/sdk/client"
	otelcontrib "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/apps/server/internal/migrations/adapters"
	"github.com/tilsley/loom/apps/server/internal/migrations/execution"
	"github.com/tilsley/loom/apps/server/internal/platform/logger"
	"github.com/tilsley/loom/apps/server/internal/platform/telemetry"
	temporalplatform "github.com/tilsley/loom/apps/server/internal/platform/temporal"
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
	//
	// Build the gRPC connection manually so the OTel stats handler injects
	// W3C traceparent metadata into every call to the Dapr sidecar.
	// This links our server spans to Dapr's own distributed trace propagation.

	daprPort := os.Getenv("DAPR_GRPC_PORT")
	if daprPort == "" {
		daprPort = "50001"
	}
	daprConn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%s", daprPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		tc.Close()
		slog.Error("dapr grpc connection failed", "error", err)
		os.Exit(1) //nolint:gocritic
	}
	defer daprConn.Close()

	daprClient := dapr.NewClientWithConnection(daprConn)
	defer daprClient.Close()

	// --- Adapters ---

	bus := adapters.NewDaprBus(daprClient, "pubsub", "migration-steps")
	store := adapters.NewDaprMigrationStore(daprClient)
	dryRunner := adapters.NewDaprDryRunAdapter(daprClient)

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

	router.Use(gin.Recovery(), otelgin.Middleware(os.Getenv("OTEL_SERVICE_NAME")), validator)
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
