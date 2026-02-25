package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// ─── POST /migrations/:id/candidates/:candidateId/start ───────────────────────

func TestStartRun_Success(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Steps:      []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
		Candidates: []api.Candidate{{Id: "billing-api"}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/start", nil)

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestStartRun_WithInputs(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Steps:      []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
		Candidates: []api.Candidate{{Id: "billing-api"}},
	}))

	inputs := map[string]string{"env": "prod"}
	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/start", api.StartRequest{
		Inputs: &inputs,
	})

	require.Equal(t, http.StatusAccepted, w.Code)
}

func TestStartRun_MigrationNotFound(t *testing.T) {
	ts := newTestServer(t)

	w := ts.do(http.MethodPost, "/migrations/unknown/candidates/billing-api/start", nil)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestStartRun_CandidateNotFound(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "other"}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/start", nil)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestStartRun_AlreadyRunning_Returns409(t *testing.T) {
	ts := newTestServer(t)
	running := api.CandidateStatusRunning
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "billing-api", Status: &running}},
		Steps:      []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/start", nil)

	require.Equal(t, http.StatusConflict, w.Code)
}

func TestStartRun_InvalidJSON_Returns400(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/start",
		bytes.NewBufferString(`{invalid json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── POST /migrations/:id/candidates/:candidateId/cancel ──────────────────────

func TestCancelRun_Returns204(t *testing.T) {
	ts := newTestServer(t)
	running := api.CandidateStatusRunning
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "billing-api", Status: &running}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/cancel", nil)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestCancelRun_MigrationNotFound_Returns404(t *testing.T) {
	ts := newTestServer(t)

	w := ts.do(http.MethodPost, "/migrations/unknown/candidates/billing-api/cancel", nil)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelRun_CandidateNotFound_Returns404(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "other"}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/cancel", nil)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelRun_CandidateNotRunning_Returns409(t *testing.T) {
	ts := newTestServer(t)
	notStarted := api.CandidateStatusNotStarted
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "billing-api", Status: &notStarted}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/cancel", nil)

	require.Equal(t, http.StatusConflict, w.Code)
}

// ─── POST /migrations/:id/candidates/:candidateId/retry-step ─────────────────

func TestRetryStep_Success(t *testing.T) {
	ts := newTestServer(t)
	running := api.CandidateStatusRunning
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "billing-api", Status: &running}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/retry-step",
		api.RetryStepRequest{StepName: "update-chart"})

	require.Equal(t, http.StatusAccepted, w.Code)
}

func TestRetryStep_MigrationNotFound_Returns404(t *testing.T) {
	ts := newTestServer(t)

	w := ts.do(http.MethodPost, "/migrations/unknown/candidates/billing-api/retry-step",
		api.RetryStepRequest{StepName: "update-chart"})

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRetryStep_CandidateNotRunning_Returns409(t *testing.T) {
	ts := newTestServer(t)
	notStarted := api.CandidateStatusNotStarted
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "billing-api", Status: &notStarted}},
	}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates/billing-api/retry-step",
		api.RetryStepRequest{StepName: "update-chart"})

	require.Equal(t, http.StatusConflict, w.Code)
}

// ─── GET /migrations/:id/candidates/:candidateId/steps ────────────────────────

func TestGetCandidateSteps_NotFound_Returns404(t *testing.T) {
	ts := newTestServer(t)
	ts.engine.getStatusFn = func(_ context.Context, id string) (*migrations.RunStatus, error) {
		return nil, migrations.RunNotFoundError{InstanceID: id}
	}

	w := ts.do(http.MethodGet, "/migrations/mig-abc/candidates/billing-api/steps", nil)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCandidateSteps_Running_Returns200(t *testing.T) {
	ts := newTestServer(t)
	ts.engine.getStatusFn = func(_ context.Context, _ string) (*migrations.RunStatus, error) {
		return &migrations.RunStatus{RuntimeStatus: "RUNNING"}, nil
	}

	w := ts.do(http.MethodGet, "/migrations/mig-abc/candidates/billing-api/steps", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp api.CandidateStepsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, api.CandidateStepsResponseStatusRunning, resp.Status)
}
