package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/pkg/api"
)

// ─── GET /migrations ──────────────────────────────────────────────────────────

func TestList_Empty(t *testing.T) {
	ts := newTestServer(t)

	w := ts.do(http.MethodGet, "/migrations", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp api.ListMigrationsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Empty(t, resp.Migrations)
}

func TestList_ReturnsMigrations(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:   "mig-abc",
		Name: "Migrate chart",
	}))

	w := ts.do(http.MethodGet, "/migrations", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp api.ListMigrationsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Migrations, 1)
	assert.Equal(t, "mig-abc", resp.Migrations[0].Id)
}

// ─── GET /migrations/:id ──────────────────────────────────────────────────────

func TestGetMigration_NotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.do(http.MethodGet, "/migrations/nonexistent", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetMigration_Found(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id: "mig-abc", Name: "Migrate chart",
	}))

	w := ts.do(http.MethodGet, "/migrations/mig-abc", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var m api.Migration
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	assert.Equal(t, "mig-abc", m.Id)
}

// ─── POST /migrations/:id/candidates ─────────────────────────────────────────

func TestSubmitCandidates_Success(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{Id: "mig-abc"}))

	w := ts.do(http.MethodPost, "/migrations/mig-abc/candidates", api.SubmitCandidatesRequest{
		Candidates: []api.Candidate{{Id: "billing-api"}, {Id: "payments-svc"}},
	})

	require.Equal(t, http.StatusNoContent, w.Code)
}

// These two tests use the validation middleware to confirm the schema contract
// is enforced end-to-end. If the middleware were removed, they would catch it.

func TestSubmitCandidates_MissingKind_Returns400(t *testing.T) {
	ts := newTestServerWithValidation(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{Id: "mig-abc"}))

	req := httptest.NewRequest(http.MethodPost, "/migrations/mig-abc/candidates",
		strings.NewReader(`{"candidates":[{"id":"billing-api"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestSubmitCandidates_WithKind_PassesValidation(t *testing.T) {
	ts := newTestServerWithValidation(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{Id: "mig-abc"}))

	req := httptest.NewRequest(http.MethodPost, "/migrations/mig-abc/candidates",
		strings.NewReader(`{"candidates":[{"id":"billing-api","kind":"application"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestSubmitCandidates_MigrationNotFound(t *testing.T) {
	ts := newTestServer(t)

	w := ts.do(http.MethodPost, "/migrations/unknown/candidates", api.SubmitCandidatesRequest{
		Candidates: []api.Candidate{{Id: "billing-api"}},
	})

	require.Equal(t, http.StatusNotFound, w.Code)
}

// ─── GET /migrations/:id/candidates ──────────────────────────────────────────

func TestGetCandidates_ReturnsList(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:         "mig-abc",
		Candidates: []api.Candidate{{Id: "billing-api"}},
	}))

	w := ts.do(http.MethodGet, "/migrations/mig-abc/candidates", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var candidates []api.Candidate
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &candidates))
	require.Len(t, candidates, 1)
	assert.Equal(t, "billing-api", candidates[0].Id)
}

// ─── POST /migrations/:id/dry-run ────────────────────────────────────────────

func TestDryRun_Success(t *testing.T) {
	ts := newTestServer(t)
	require.NoError(t, ts.store.Save(context.Background(), api.Migration{
		Id:    "mig-abc",
		Steps: []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
	}))
	ts.dryRun.result = &api.DryRunResult{
		Steps: []api.StepDryRunResult{{StepName: "update-chart", Skipped: false}},
	}

	w := ts.do(http.MethodPost, "/migrations/mig-abc/dry-run", api.DryRunCandidateRequest{
		Candidate: api.Candidate{Id: "billing-api"},
	})

	require.Equal(t, http.StatusOK, w.Code)
	var result api.DryRunResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result.Steps, 1)
}

func TestDryRun_MigrationNotFound(t *testing.T) {
	ts := newTestServer(t)

	w := ts.do(http.MethodPost, "/migrations/unknown/dry-run", api.DryRunCandidateRequest{
		Candidate: api.Candidate{Id: "billing-api"},
	})

	require.Equal(t, http.StatusNotFound, w.Code)
}
