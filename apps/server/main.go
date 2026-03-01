package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.temporal.io/sdk/client"
	otelcontrib "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/apps/server/internal/migrations/execution"
	"github.com/tilsley/loom/apps/server/internal/migrations/handler"
	"github.com/tilsley/loom/apps/server/internal/migrations/migrator"
	"github.com/tilsley/loom/apps/server/internal/migrations/store"
	"github.com/tilsley/loom/apps/server/internal/migrations/store/pgmigrations"
	"github.com/tilsley/loom/apps/server/internal/platform/logger"
	pgplatform "github.com/tilsley/loom/apps/server/internal/platform/postgres"
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

	// --- Platform: Redis ---

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	defer rdb.Close() //nolint:errcheck

	// --- Platform: Postgres (optional — event store) ---

	var eventStore migrations.EventStore
	if pgURL := os.Getenv("POSTGRES_URL"); pgURL != "" {
		pool, err := pgplatform.New(ctx, pgURL, pgmigrations.FS)
		if err != nil {
			slog.Error("postgres init failed", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		eventStore = store.NewPGEventStore(pool)
		slog.Info("event store enabled (postgres)")
	} else {
		slog.Info("event store disabled (no POSTGRES_URL)")
	}

	// --- Adapters ---

	migrationStore := store.NewRedisMigrationStore(rdb)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	notifier := migrator.NewHTTPMigratorNotifier(httpClient)
	dryRunner := migrator.NewHTTPDryRunAdapter(httpClient)

	// --- Temporal Worker ---

	activities := execution.NewActivities(notifier, migrationStore, eventStore, slog)

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

	svc := migrations.NewService(engine, migrationStore, dryRunner, eventStore)

	router := gin.New()

	validator, err := validation.New(schemas.OpenAPISpec)
	if err != nil {
		slog.Error("openapi validation middleware init failed", "error", err)
		os.Exit(1)
	}

	router.Use(gin.Logger(), gin.Recovery(), otelgin.Middleware(os.Getenv("OTEL_SERVICE_NAME")), validator)
	handler.RegisterRoutes(router, svc, slog)

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
