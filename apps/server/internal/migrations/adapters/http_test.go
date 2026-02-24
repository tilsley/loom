package adapters_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/apps/server/internal/migrations/adapters"
	"github.com/tilsley/loom/apps/server/internal/platform/validation"
	"github.com/tilsley/loom/pkg/api"
	"github.com/tilsley/loom/schemas"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func ptr[T any](v T) *T { return &v }

// ─── Stubs ────────────────────────────────────────────────────────────────────

type stubEngine struct {
	startFn      func(ctx context.Context, name, id string, input any) (string, error)
	getStatusFn  func(ctx context.Context, id string) (*migrations.WorkflowStatus, error)
	raiseEventFn func(ctx context.Context, id, event string, payload any) error
	cancelFn     func(ctx context.Context, id string) error
}

func (e *stubEngine) StartWorkflow(ctx context.Context, name, id string, input any) (string, error) {
	if e.startFn != nil {
		return e.startFn(ctx, name, id, input)
	}
	return id, nil
}

func (e *stubEngine) GetStatus(ctx context.Context, id string) (*migrations.WorkflowStatus, error) {
	if e.getStatusFn != nil {
		return e.getStatusFn(ctx, id)
	}
	return &migrations.WorkflowStatus{RuntimeStatus: "RUNNING"}, nil
}

func (e *stubEngine) RaiseEvent(ctx context.Context, id, event string, payload any) error {
	if e.raiseEventFn != nil {
		return e.raiseEventFn(ctx, id, event, payload)
	}
	return nil
}

func (e *stubEngine) CancelWorkflow(ctx context.Context, id string) error {
	if e.cancelFn != nil {
		return e.cancelFn(ctx, id)
	}
	return nil
}

type stubDryRunner struct {
	result *api.DryRunResult
	err    error
}

func (d *stubDryRunner) DryRun(_ context.Context, _ string, _ api.DryRunRequest) (*api.DryRunResult, error) {
	return d.result, d.err
}

type memStore struct {
	migrations  map[string]api.Migration
	candidates  map[string][]api.Candidate
	setStatusFn func(ctx context.Context, migID, candidateID string, status api.CandidateStatus) error
}

func newMemStore() *memStore {
	return &memStore{
		migrations: make(map[string]api.Migration),
		candidates: make(map[string][]api.Candidate),
	}
}

func (m *memStore) Save(_ context.Context, mig api.Migration) error {
	m.migrations[mig.Id] = mig
	return nil
}

func (m *memStore) Get(_ context.Context, id string) (*api.Migration, error) {
	mig, ok := m.migrations[id]
	if !ok {
		return nil, nil
	}
	return &mig, nil
}

func (m *memStore) List(_ context.Context) ([]api.Migration, error) {
	out := make([]api.Migration, 0, len(m.migrations))
	for _, mig := range m.migrations {
		out = append(out, mig)
	}
	return out, nil
}

func (m *memStore) SetCandidateStatus(ctx context.Context, migID, candidateID string, status api.CandidateStatus) error {
	if m.setStatusFn != nil {
		return m.setStatusFn(ctx, migID, candidateID, status)
	}
	mig, ok := m.migrations[migID]
	if !ok {
		return nil
	}
	for i, c := range mig.Candidates {
		if c.Id == candidateID {
			mig.Candidates[i].Status = &status
			m.migrations[migID] = mig
			return nil
		}
	}
	return nil
}

func (m *memStore) SaveCandidates(_ context.Context, migID string, candidates []api.Candidate) error {
	m.candidates[migID] = candidates
	return nil
}

func (m *memStore) GetCandidates(_ context.Context, migID string) ([]api.Candidate, error) {
	mig, ok := m.migrations[migID]
	if ok && len(mig.Candidates) > 0 {
		return mig.Candidates, nil
	}
	return m.candidates[migID], nil
}

// ─── Test server builder ──────────────────────────────────────────────────────

type testServer struct {
	router *gin.Engine
	store  *memStore
	engine *stubEngine
	dryRun *stubDryRunner
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ts := &testServer{
		store:  newMemStore(),
		engine: &stubEngine{},
		dryRun: &stubDryRunner{},
	}
	svc := migrations.NewService(ts.engine, ts.store, ts.dryRun)
	r := gin.New()
	adapters.RegisterRoutes(r, svc, slog.Default())
	ts.router = r
	return ts
}

func newTestServerWithValidation(t *testing.T) *testServer {
	t.Helper()
	ts := newTestServer(t)
	mw, err := validation.New(schemas.OpenAPISpec)
	require.NoError(t, err)
	r := gin.New()
	r.Use(mw)
	adapters.RegisterRoutes(r, migrations.NewService(ts.engine, ts.store, ts.dryRun), slog.Default())
	ts.router = r
	return ts
}

func (ts *testServer) do(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)
	return w
}

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

// ─── GET /migrations/:id/candidates/:candidateId/steps ────────────────────────

func TestGetCandidateSteps_NotFound_Returns404(t *testing.T) {
	ts := newTestServer(t)
	ts.engine.getStatusFn = func(_ context.Context, id string) (*migrations.WorkflowStatus, error) {
		return nil, migrations.WorkflowNotFoundError{InstanceID: id}
	}

	w := ts.do(http.MethodGet, "/migrations/mig-abc/candidates/billing-api/steps", nil)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCandidateSteps_Running_Returns200(t *testing.T) {
	ts := newTestServer(t)
	ts.engine.getStatusFn = func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
		return &migrations.WorkflowStatus{RuntimeStatus: "RUNNING"}, nil
	}

	w := ts.do(http.MethodGet, "/migrations/mig-abc/candidates/billing-api/steps", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp api.CandidateStepsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, api.CandidateStepsResponseStatusRunning, resp.Status)
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

// ─── POST /event/:id ─────────────────────────────────────────────────────────

func TestEvent_RaisesSignal(t *testing.T) {
	ts := newTestServer(t)
	var raised string
	ts.engine.raiseEventFn = func(_ context.Context, _, event string, _ any) error {
		raised = event
		return nil
	}

	w := ts.do(http.MethodPost, "/event/run-123", api.StepCompletedEvent{
		StepName:    "update-chart",
		CandidateId: "billing-api",
		Success:     true,
	})

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.NotEmpty(t, raised)
}

func TestEvent_BadJSON_Returns400(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/event/run-123",
		bytes.NewBufferString(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── POST /registry/announce ─────────────────────────────────────────────────

func TestAnnounce_DirectJSON(t *testing.T) {
	ts := newTestServer(t)

	ann := api.MigrationAnnouncement{
		Id:          "migrate-chart",
		Name:        "Migrate chart",
		Description: "Upgrades Helm charts",
		Steps:       []api.StepDefinition{{Name: "update-chart", WorkerApp: "app-chart-migrator"}},
		WorkerUrl:   "http://app-chart-migrator:3001",
	}

	w := ts.do(http.MethodPost, "/registry/announce", ann)

	require.Equal(t, http.StatusOK, w.Code)
	// Migration should be persisted in the store.
	m, err := ts.store.Get(context.Background(), "migrate-chart")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "Migrate chart", m.Name)
}
