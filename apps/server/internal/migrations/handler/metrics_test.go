package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations"
)

func TestMetricsOverview(t *testing.T) {
	t.Run("returns empty overview when no event store", func(t *testing.T) {
		ts := newTestServer(t)
		w := ts.do("GET", "/metrics/overview", nil)
		require.Equal(t, http.StatusOK, w.Code)

		var overview migrations.MetricsOverview
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &overview))
		assert.Equal(t, 0, overview.TotalRuns)
		assert.Equal(t, 0, overview.CompletedRuns)
		assert.Equal(t, 0, overview.PRsRaised)
		assert.Equal(t, float64(0), overview.AvgDurationMs)
		assert.Equal(t, float64(0), overview.FailureRate)
	})
}

func TestMetricsSteps(t *testing.T) {
	t.Run("returns empty steps when no event store", func(t *testing.T) {
		ts := newTestServer(t)
		w := ts.do("GET", "/metrics/steps", nil)
		require.Equal(t, http.StatusOK, w.Code)

		var steps []migrations.StepMetrics
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &steps))
		assert.Empty(t, steps)
	})
}

func TestMetricsTimeline(t *testing.T) {
	t.Run("returns empty timeline when no event store", func(t *testing.T) {
		ts := newTestServer(t)
		w := ts.do("GET", "/metrics/timeline", nil)
		require.Equal(t, http.StatusOK, w.Code)

		var timeline []migrations.TimelinePoint
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &timeline))
		assert.Empty(t, timeline)
	})

	t.Run("accepts custom days parameter", func(t *testing.T) {
		ts := newTestServer(t)
		w := ts.do("GET", "/metrics/timeline?days=7", nil)
		require.Equal(t, http.StatusOK, w.Code)
	})
}

func TestMetricsFailures(t *testing.T) {
	t.Run("returns empty failures when no event store", func(t *testing.T) {
		ts := newTestServer(t)
		w := ts.do("GET", "/metrics/failures", nil)
		require.Equal(t, http.StatusOK, w.Code)

		var failures []migrations.StepEvent
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &failures))
		assert.Empty(t, failures)
	})

	t.Run("accepts custom limit parameter", func(t *testing.T) {
		ts := newTestServer(t)
		w := ts.do("GET", "/metrics/failures?limit=5", nil)
		require.Equal(t, http.StatusOK, w.Code)
	})
}
