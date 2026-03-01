package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/tilsley/loom/apps/server/internal/migrations"
)

const instrName = "github.com/tilsley/loom"

// PGEventStore implements migrations.EventStore backed by PostgreSQL.
type PGEventStore struct {
	pool *pgxpool.Pool

	// OTel business metrics emitted on every RecordEvent call.
	stepDuration metric.Float64Histogram
	stepComplete metric.Int64Counter
	stepRetried  metric.Int64Counter
	runDuration  metric.Float64Histogram
	prsRaised    metric.Int64Counter
}

// NewPGEventStore creates a new PGEventStore with the given connection pool.
func NewPGEventStore(pool *pgxpool.Pool) *PGEventStore {
	m := otel.Meter(instrName)

	stepDuration, _ := m.Float64Histogram("loom.step.duration",
		metric.WithDescription("Step execution duration in milliseconds"),
		metric.WithUnit("ms"))
	stepComplete, _ := m.Int64Counter("loom.step.completed",
		metric.WithDescription("Number of steps completed"))
	stepRetried, _ := m.Int64Counter("loom.step.retried",
		metric.WithDescription("Number of steps retried"))
	runDuration, _ := m.Float64Histogram("loom.run.duration",
		metric.WithDescription("Run duration in milliseconds"),
		metric.WithUnit("ms"))
	prsRaised, _ := m.Int64Counter("loom.prs.raised",
		metric.WithDescription("Number of PRs raised"))

	return &PGEventStore{
		pool:         pool,
		stepDuration: stepDuration,
		stepComplete: stepComplete,
		stepRetried:  stepRetried,
		runDuration:  runDuration,
		prsRaised:    prsRaised,
	}
}

// RecordEvent inserts an event row and emits OTel business metrics.
func (s *PGEventStore) RecordEvent(ctx context.Context, event migrations.StepEvent) error {
	var metadataJSON []byte
	if event.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO step_events (migration_id, candidate_id, step_name, event_type, status, duration_ms, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		event.MigrationID, event.CandidateID, nilIfEmpty(event.StepName),
		event.EventType, nilIfEmpty(event.Status), event.DurationMs, metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("insert step_event: %w", err)
	}

	// Emit OTel metrics based on event type.
	s.emitMetrics(ctx, event)

	return nil
}

func (s *PGEventStore) emitMetrics(ctx context.Context, event migrations.StepEvent) {
	attrs := []attribute.KeyValue{
		attribute.String("step_name", event.StepName),
	}

	switch event.EventType {
	case migrations.EventStepCompleted:
		s.stepComplete.Add(ctx, 1, metric.WithAttributes(
			append(attrs, attribute.String("status", event.Status))...,
		))
		if event.DurationMs != nil {
			s.stepDuration.Record(ctx, float64(*event.DurationMs), metric.WithAttributes(attrs...))
		}
	case migrations.EventStepRetried:
		s.stepRetried.Add(ctx, 1, metric.WithAttributes(attrs...))
	case migrations.EventRunCompleted:
		if event.DurationMs != nil {
			s.runDuration.Record(ctx, float64(*event.DurationMs), metric.WithAttributes(
				attribute.String("migration_id", event.MigrationID),
			))
		}
	}

	// Count PRs based on metadata.
	if event.Metadata != nil {
		if _, ok := event.Metadata["prUrl"]; ok {
			s.prsRaised.Add(ctx, 1)
		}
	}
}

// GetOverview returns aggregate totals for the metrics dashboard.
func (s *PGEventStore) GetOverview(ctx context.Context) (*migrations.MetricsOverview, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE event_type = 'run_started'),
			COUNT(*) FILTER (WHERE event_type = 'run_completed'),
			COUNT(*) FILTER (WHERE event_type = 'step_completed' AND status = 'failed'),
			COUNT(*) FILTER (WHERE metadata->>'prUrl' IS NOT NULL),
			COALESCE(AVG(duration_ms) FILTER (WHERE event_type = 'step_completed' AND duration_ms IS NOT NULL), 0),
			CASE
				WHEN COUNT(*) FILTER (WHERE event_type = 'step_completed') = 0 THEN 0
				ELSE COUNT(*) FILTER (WHERE event_type = 'step_completed' AND status = 'failed')::float
					/ COUNT(*) FILTER (WHERE event_type = 'step_completed')
			END
		FROM step_events
	`)

	var o migrations.MetricsOverview
	err := row.Scan(&o.TotalRuns, &o.CompletedRuns, &o.FailedSteps, &o.PRsRaised, &o.AvgDurationMs, &o.FailureRate)
	if err != nil {
		return nil, fmt.Errorf("overview query: %w", err)
	}
	return &o, nil
}

// GetStepMetrics returns per-step-name aggregated statistics.
func (s *PGEventStore) GetStepMetrics(ctx context.Context) ([]migrations.StepMetrics, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			step_name,
			COUNT(*),
			COALESCE(AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL), 0),
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms IS NOT NULL), 0),
			CASE
				WHEN COUNT(*) = 0 THEN 0
				ELSE COUNT(*) FILTER (WHERE status = 'failed')::float / COUNT(*)
			END
		FROM step_events
		WHERE event_type = 'step_completed' AND step_name IS NOT NULL
		GROUP BY step_name
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("step metrics query: %w", err)
	}
	defer rows.Close()

	var result []migrations.StepMetrics
	for rows.Next() {
		var sm migrations.StepMetrics
		if err := rows.Scan(&sm.StepName, &sm.Count, &sm.AvgMs, &sm.P95Ms, &sm.FailureRate); err != nil {
			return nil, fmt.Errorf("scan step metrics: %w", err)
		}
		result = append(result, sm)
	}
	return result, rows.Err()
}

// GetTimeline returns daily event counts for the specified number of days.
func (s *PGEventStore) GetTimeline(ctx context.Context, days int) ([]migrations.TimelinePoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			d::date::text AS date,
			COUNT(*) FILTER (WHERE event_type = 'run_started'),
			COUNT(*) FILTER (WHERE event_type = 'run_completed'),
			COUNT(*) FILTER (WHERE event_type = 'step_completed' AND status = 'failed')
		FROM generate_series(
			NOW() - ($1 || ' days')::interval,
			NOW(),
			'1 day'::interval
		) AS d
		LEFT JOIN step_events ON date_trunc('day', step_events.created_at) = d::date
		GROUP BY d::date
		ORDER BY d::date
	`, days)
	if err != nil {
		return nil, fmt.Errorf("timeline query: %w", err)
	}
	defer rows.Close()

	var result []migrations.TimelinePoint
	for rows.Next() {
		var tp migrations.TimelinePoint
		if err := rows.Scan(&tp.Date, &tp.Started, &tp.Completed, &tp.Failed); err != nil {
			return nil, fmt.Errorf("scan timeline: %w", err)
		}
		result = append(result, tp)
	}
	return result, rows.Err()
}

// GetRecentFailures returns the most recent failed step events.
func (s *PGEventStore) GetRecentFailures(ctx context.Context, limit int) ([]migrations.StepEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, migration_id, candidate_id, step_name, event_type, status, duration_ms, metadata, created_at
		FROM step_events
		WHERE event_type = 'step_completed' AND status = 'failed'
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent failures query: %w", err)
	}
	defer rows.Close()

	var result []migrations.StepEvent
	for rows.Next() {
		var e migrations.StepEvent
		var metadataJSON []byte
		var stepName, status *string
		var durationMs *int
		if err := rows.Scan(&e.ID, &e.MigrationID, &e.CandidateID, &stepName, &e.EventType, &status, &durationMs, &metadataJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan failure: %w", err)
		}
		if stepName != nil {
			e.StepName = *stepName
		}
		if status != nil {
			e.Status = *status
		}
		e.DurationMs = durationMs
		if metadataJSON != nil {
			_ = json.Unmarshal(metadataJSON, &e.Metadata)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// Compile-time check.
var _ migrations.EventStore = (*PGEventStore)(nil)

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Ensure pgx is imported for row scanning.
var _ = pgx.ErrNoRows
