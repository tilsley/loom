package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/apps/server/internal/migrations/handler"
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
	getStatusFn  func(ctx context.Context, id string) (*migrations.RunStatus, error)
	raiseEventFn func(ctx context.Context, id, event string, payload any) error
	cancelFn     func(ctx context.Context, id string) error
}

func (e *stubEngine) StartRun(ctx context.Context, name, id string, input any) (string, error) {
	if e.startFn != nil {
		return e.startFn(ctx, name, id, input)
	}
	return id, nil
}

func (e *stubEngine) GetStatus(ctx context.Context, id string) (*migrations.RunStatus, error) {
	if e.getStatusFn != nil {
		return e.getStatusFn(ctx, id)
	}
	return &migrations.RunStatus{RuntimeStatus: "RUNNING"}, nil
}

func (e *stubEngine) RaiseEvent(ctx context.Context, id, event string, payload any) error {
	if e.raiseEventFn != nil {
		return e.raiseEventFn(ctx, id, event, payload)
	}
	return nil
}

func (e *stubEngine) CancelRun(ctx context.Context, id string) error {
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

func (m *memStore) UpdateCandidateMetadata(_ context.Context, migID, candidateID string, metadata map[string]string) error {
	mig, ok := m.migrations[migID]
	if !ok {
		return nil
	}
	for i, c := range mig.Candidates {
		if c.Id == candidateID {
			if c.Metadata == nil {
				md := map[string]string{}
				mig.Candidates[i].Metadata = &md
			}
			for k, v := range metadata {
				(*mig.Candidates[i].Metadata)[k] = v
			}
			m.migrations[migID] = mig
			return nil
		}
	}
	return nil
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
	svc := migrations.NewService(ts.engine, ts.store, ts.dryRun, nil)
	r := gin.New()
	handler.RegisterRoutes(r, svc, slog.Default())
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
	handler.RegisterRoutes(r, migrations.NewService(ts.engine, ts.store, ts.dryRun, nil), slog.Default())
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

