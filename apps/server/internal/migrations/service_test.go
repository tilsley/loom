package migrations_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// Compile-time interface compliance checks.
var (
	_ migrations.MigrationStore = (*memStore)(nil)
	_ migrations.WorkflowEngine = (*stubEngine)(nil)
	_ migrations.DryRunner      = (*stubDryRunner)(nil)
)

func ptr[T any](v T) *T { return &v }

// ─── stubEngine ───────────────────────────────────────────────────────────────

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

// ─── stubDryRunner ────────────────────────────────────────────────────────────

type stubDryRunner struct {
	result  *api.DryRunResult
	err     error
	lastReq api.DryRunRequest
}

func (d *stubDryRunner) DryRun(ctx context.Context, req api.DryRunRequest) (*api.DryRunResult, error) {
	d.lastReq = req
	return d.result, d.err
}

// ─── memStore ─────────────────────────────────────────────────────────────────

type memStore struct {
	data              map[string]api.RegisteredMigration
	candidates        map[string][]api.CandidateWithStatus
	cancelledAttempts map[string][]api.CancelledAttempt

	// per-method error stubs
	errSave                   error
	errGet                    error
	errList                   error
	errDelete                 error
	errAppendCancelledAttempt error
	errSetCandidateRun        error
	errDeleteCandidateRun     error
	errSaveCandidates         error
	errGetCandidates          error
}

func newMemStore() *memStore {
	return &memStore{
		data:              make(map[string]api.RegisteredMigration),
		candidates:        make(map[string][]api.CandidateWithStatus),
		cancelledAttempts: make(map[string][]api.CancelledAttempt),
	}
}

func (s *memStore) Save(_ context.Context, m api.RegisteredMigration) error {
	if s.errSave != nil {
		return s.errSave
	}
	s.data[m.Id] = m
	return nil
}

func (s *memStore) Get(_ context.Context, id string) (*api.RegisteredMigration, error) {
	if s.errGet != nil {
		return nil, s.errGet
	}
	m, ok := s.data[id]
	if !ok {
		return nil, nil
	}
	return &m, nil
}

func (s *memStore) List(_ context.Context) ([]api.RegisteredMigration, error) {
	if s.errList != nil {
		return nil, s.errList
	}
	out := make([]api.RegisteredMigration, 0, len(s.data))
	for _, m := range s.data {
		out = append(out, m)
	}
	return out, nil
}

func (s *memStore) Delete(_ context.Context, id string) error {
	if s.errDelete != nil {
		return s.errDelete
	}
	delete(s.data, id)
	return nil
}

func (s *memStore) AppendCancelledAttempt(_ context.Context, migrationID string, attempt api.CancelledAttempt) error {
	if s.errAppendCancelledAttempt != nil {
		return s.errAppendCancelledAttempt
	}
	s.cancelledAttempts[migrationID] = append(s.cancelledAttempts[migrationID], attempt)
	return nil
}

func (s *memStore) SetCandidateRun(_ context.Context, migrationID, candidateID string, run api.CandidateRun) error {
	if s.errSetCandidateRun != nil {
		return s.errSetCandidateRun
	}
	m, ok := s.data[migrationID]
	if !ok {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	if m.CandidateRuns == nil {
		runs := make(map[string]api.CandidateRun)
		m.CandidateRuns = &runs
	}
	(*m.CandidateRuns)[candidateID] = run
	s.data[migrationID] = m
	return nil
}

func (s *memStore) DeleteCandidateRun(_ context.Context, migrationID, candidateID string) error {
	if s.errDeleteCandidateRun != nil {
		return s.errDeleteCandidateRun
	}
	m, ok := s.data[migrationID]
	if !ok {
		return nil
	}
	if m.CandidateRuns != nil {
		delete(*m.CandidateRuns, candidateID)
	}
	s.data[migrationID] = m
	return nil
}

func (s *memStore) SaveCandidates(_ context.Context, migrationID string, candidates []api.Candidate) error {
	if s.errSaveCandidates != nil {
		return s.errSaveCandidates
	}
	cs := make([]api.CandidateWithStatus, len(candidates))
	for i, c := range candidates {
		cs[i] = api.CandidateWithStatus{
			Id:       c.Id,
			Kind:     c.Kind,
			Metadata: c.Metadata,
			State:    c.State,
			Files:    c.Files,
			Status:   api.CandidateStatusNotStarted,
		}
	}
	s.candidates[migrationID] = cs
	return nil
}

func (s *memStore) GetCandidates(_ context.Context, migrationID string) ([]api.CandidateWithStatus, error) {
	if s.errGetCandidates != nil {
		return nil, s.errGetCandidates
	}
	return s.candidates[migrationID], nil
}

// ─── constructor helper ───────────────────────────────────────────────────────

func newSvc(store *memStore, engine *stubEngine, dr *stubDryRunner) *migrations.Service {
	return migrations.NewService(engine, store, dr, "test-org")
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestService_Register(t *testing.T) {
	t.Run("returns migration with generated ID and org", func(t *testing.T) {
		store := newMemStore()
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		m, err := svc.Register(context.Background(), api.RegisterMigrationRequest{
			Name:  "Test Migration",
			Steps: []api.StepDefinition{{Name: "step-1", WorkerApp: "app"}},
		})

		require.NoError(t, err)
		assert.NotEmpty(t, m.Id)
		assert.Equal(t, "Test Migration", m.Name)
		assert.Equal(t, ptr("test-org"), m.Org)
		assert.WithinDuration(t, time.Now(), m.CreatedAt, 2*time.Second)

		// persisted to the store
		stored, _ := store.Get(context.Background(), m.Id)
		require.NotNil(t, stored)
		assert.Equal(t, m.Id, stored.Id)
	})

	t.Run("propagates store save error", func(t *testing.T) {
		store := newMemStore()
		store.errSave = errors.New("disk full")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.Register(context.Background(), api.RegisterMigrationRequest{Name: "x"})
		require.ErrorContains(t, err, "disk full")
	})
}

func TestService_Announce(t *testing.T) {
	t.Run("creates new migration when not found", func(t *testing.T) {
		store := newMemStore()
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		m, err := svc.Announce(context.Background(), api.MigrationAnnouncement{
			Id:   "app-chart-migration",
			Name: "App Chart Migration",
		})

		require.NoError(t, err)
		assert.Equal(t, "app-chart-migration", m.Id)
		assert.Equal(t, ptr("test-org"), m.Org)
		assert.WithinDuration(t, time.Now(), m.CreatedAt, 2*time.Second)
	})

	t.Run("upserts existing migration preserving createdAt and candidates", func(t *testing.T) {
		store := newMemStore()
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})
		ctx := context.Background()

		createdAt := time.Now().Add(-24 * time.Hour).UTC()
		_ = store.Save(ctx, api.RegisteredMigration{
			Id:         "app-chart-migration",
			Name:       "Old Name",
			CreatedAt:  createdAt,
			Candidates: []api.Candidate{{Id: "repo-a"}},
		})

		m, err := svc.Announce(ctx, api.MigrationAnnouncement{
			Id:    "app-chart-migration",
			Name:  "App Chart Migration",
			Steps: []api.StepDefinition{{Name: "step-1", WorkerApp: "app"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "App Chart Migration", m.Name)
		assert.Equal(t, createdAt, m.CreatedAt, "createdAt must be preserved")
		assert.Len(t, m.Candidates, 1, "candidates must be preserved")
		assert.Len(t, m.Steps, 1, "steps must be updated")
		assert.Equal(t, ptr("test-org"), m.Org)
	})

	t.Run("propagates store Get error", func(t *testing.T) {
		store := newMemStore()
		store.errGet = errors.New("connection refused")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.Announce(context.Background(), api.MigrationAnnouncement{Id: "x"})
		require.ErrorContains(t, err, "connection refused")
	})

	t.Run("propagates store Save error", func(t *testing.T) {
		store := newMemStore()
		store.errSave = errors.New("write failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.Announce(context.Background(), api.MigrationAnnouncement{Id: "new"})
		require.ErrorContains(t, err, "write failed")
	})
}

func TestService_List(t *testing.T) {
	t.Run("returns all migrations from store", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(context.Background(), api.RegisteredMigration{Id: "m1", Name: "M1"})
		_ = store.Save(context.Background(), api.RegisteredMigration{Id: "m2", Name: "M2"})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		list, err := svc.List(context.Background())
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})

	t.Run("propagates store error", func(t *testing.T) {
		store := newMemStore()
		store.errList = errors.New("list failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.List(context.Background())
		require.ErrorContains(t, err, "list failed")
	})
}

func TestService_Get(t *testing.T) {
	t.Run("returns migration by ID", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(context.Background(), api.RegisteredMigration{Id: "m1", Name: "M1"})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		m, err := svc.Get(context.Background(), "m1")
		require.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, "M1", m.Name)
	})

	t.Run("returns nil for unknown ID", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		m, err := svc.Get(context.Background(), "unknown")
		require.NoError(t, err)
		assert.Nil(t, m)
	})

	t.Run("propagates store error", func(t *testing.T) {
		store := newMemStore()
		store.errGet = errors.New("get failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.Get(context.Background(), "m1")
		require.ErrorContains(t, err, "get failed")
	})
}

func TestService_DeleteMigration(t *testing.T) {
	t.Run("removes migration from store", func(t *testing.T) {
		store := newMemStore()
		ctx := context.Background()
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1"})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		require.NoError(t, svc.DeleteMigration(ctx, "m1"))

		m, _ := store.Get(ctx, "m1")
		assert.Nil(t, m)
	})

	t.Run("propagates store error", func(t *testing.T) {
		store := newMemStore()
		store.errDelete = errors.New("delete failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.DeleteMigration(context.Background(), "m1")
		require.ErrorContains(t, err, "delete failed")
	})
}

func TestService_SubmitCandidates(t *testing.T) {
	t.Run("saves candidates for known migration", func(t *testing.T) {
		store := newMemStore()
		ctx := context.Background()
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1"})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.SubmitCandidates(ctx, "m1", api.SubmitCandidatesRequest{
			Candidates: []api.Candidate{{Id: "repo-a"}, {Id: "repo-b"}},
		})
		require.NoError(t, err)

		cs, _ := store.GetCandidates(ctx, "m1")
		assert.Len(t, cs, 2)
	})

	t.Run("returns error when migration not found", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		err := svc.SubmitCandidates(context.Background(), "missing", api.SubmitCandidatesRequest{})
		require.ErrorContains(t, err, "not found")
	})

	t.Run("propagates store Get error", func(t *testing.T) {
		store := newMemStore()
		store.errGet = errors.New("get failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.SubmitCandidates(context.Background(), "m1", api.SubmitCandidatesRequest{})
		require.ErrorContains(t, err, "get failed")
	})
}

func TestService_GetCandidates(t *testing.T) {
	const runID = "m1__repo-a"

	t.Run("returns candidates unchanged when none are running", func(t *testing.T) {
		store := newMemStore()
		store.candidates["m1"] = []api.CandidateWithStatus{
			{Id: "repo-a", Status: api.CandidateStatusNotStarted},
			{Id: "repo-b", Status: api.CandidateStatusCompleted},
		}
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		assert.Len(t, cs, 2)
		assert.Equal(t, api.CandidateStatusNotStarted, cs[0].Status)
	})

	t.Run("running candidate with live workflow is unchanged", func(t *testing.T) {
		store := newMemStore()
		store.candidates["m1"] = []api.CandidateWithStatus{
			{Id: "repo-a", Status: api.CandidateStatusRunning, RunId: ptr(runID)},
		}
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
				return &migrations.WorkflowStatus{RuntimeStatus: "RUNNING"}, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		assert.Equal(t, api.CandidateStatusRunning, cs[0].Status)
		assert.NotNil(t, cs[0].RunId)
	})

	t.Run("stale running candidate is reset to not_started", func(t *testing.T) {
		store := newMemStore()
		store.candidates["m1"] = []api.CandidateWithStatus{
			{Id: "repo-a", Status: api.CandidateStatusRunning, RunId: ptr(runID)},
		}
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, id string) (*migrations.WorkflowStatus, error) {
				return nil, migrations.WorkflowNotFoundError{InstanceID: id}
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		assert.Equal(t, api.CandidateStatusNotStarted, cs[0].Status)
		assert.Nil(t, cs[0].RunId)
	})

	t.Run("non-not-found engine error leaves candidate unchanged", func(t *testing.T) {
		store := newMemStore()
		store.candidates["m1"] = []api.CandidateWithStatus{
			{Id: "repo-a", Status: api.CandidateStatusRunning, RunId: ptr(runID)},
		}
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
				return nil, errors.New("connection timeout")
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		assert.Equal(t, api.CandidateStatusRunning, cs[0].Status, "should not reset on non-not-found error")
	})

	t.Run("propagates store error", func(t *testing.T) {
		store := newMemStore()
		store.errGetCandidates = errors.New("store unavailable")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.GetCandidates(context.Background(), "m1")
		require.ErrorContains(t, err, "store unavailable")
	})
}

func TestService_GetRunInfo(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid run ID returns nil without error", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		info, err := svc.GetRunInfo(ctx, "no-separator-here")
		require.NoError(t, err)
		assert.Nil(t, info)
	})

	t.Run("migration not found returns nil without error", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		info, err := svc.GetRunInfo(ctx, "missing__repo-a")
		require.NoError(t, err)
		assert.Nil(t, info)
	})

	t.Run("migration with no candidate runs returns nil", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1"})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		info, err := svc.GetRunInfo(ctx, "m1__repo-a")
		require.NoError(t, err)
		assert.Nil(t, info)
	})

	t.Run("candidate run not in map returns nil", func(t *testing.T) {
		store := newMemStore()
		runs := map[string]api.CandidateRun{"other": {Status: api.CandidateRunStatusRunning}}
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1", CandidateRuns: &runs})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		info, err := svc.GetRunInfo(ctx, "m1__repo-a")
		require.NoError(t, err)
		assert.Nil(t, info)
	})

	t.Run("running candidate run returns RunInfo with running status", func(t *testing.T) {
		store := newMemStore()
		runs := map[string]api.CandidateRun{"repo-a": {Status: api.CandidateRunStatusRunning}}
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1", CandidateRuns: &runs})
		store.candidates["m1"] = []api.CandidateWithStatus{{Id: "repo-a", Status: api.CandidateStatusRunning}}
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		info, err := svc.GetRunInfo(ctx, "m1__repo-a")
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, api.RunInfoStatusRunning, info.Status)
		assert.Equal(t, "m1__repo-a", info.RunId)
		assert.Equal(t, "m1", info.MigrationId)
		assert.Equal(t, "repo-a", info.Candidate.Id)
	})

	t.Run("completed candidate run returns RunInfo with completed status", func(t *testing.T) {
		store := newMemStore()
		runs := map[string]api.CandidateRun{"repo-a": {Status: api.CandidateRunStatusCompleted}}
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1", CandidateRuns: &runs})
		store.candidates["m1"] = []api.CandidateWithStatus{{Id: "repo-a", Status: api.CandidateStatusCompleted}}
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		info, err := svc.GetRunInfo(ctx, "m1__repo-a")
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, api.RunInfoStatusCompleted, info.Status)
	})

	t.Run("propagates store Get error", func(t *testing.T) {
		store := newMemStore()
		store.errGet = errors.New("store down")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.GetRunInfo(ctx, "m1__repo-a")
		require.ErrorContains(t, err, "store down")
	})
}

func TestService_Cancel(t *testing.T) {
	ctx := context.Background()

	setup := func(store *memStore) {
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1"})
	}

	t.Run("cancels workflow, writes audit entry, resets candidate", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		var cancelledID string
		engine := &stubEngine{
			cancelFn: func(_ context.Context, id string) error {
				cancelledID = id
				return nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1__repo-a")
		require.NoError(t, err)
		assert.Equal(t, "m1__repo-a", cancelledID)
		require.Len(t, store.cancelledAttempts["m1"], 1)
		assert.Equal(t, "repo-a", store.cancelledAttempts["m1"][0].CandidateId)
		assert.Equal(t, "m1__repo-a", store.cancelledAttempts["m1"][0].RunId)
	})

	t.Run("workflow not found is tolerated; audit and reset still proceed", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		engine := &stubEngine{
			cancelFn: func(_ context.Context, id string) error {
				return migrations.WorkflowNotFoundError{InstanceID: id}
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1__repo-a")
		require.NoError(t, err)
		assert.Len(t, store.cancelledAttempts["m1"], 1, "audit entry must still be written")
	})

	t.Run("other engine error is returned immediately", func(t *testing.T) {
		store := newMemStore()
		engine := &stubEngine{
			cancelFn: func(_ context.Context, _ string) error {
				return errors.New("temporal unavailable")
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1__repo-a")
		require.ErrorContains(t, err, "temporal unavailable")
	})

	t.Run("invalid run ID returns error", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		err := svc.Cancel(ctx, "no-separator")
		require.Error(t, err)
	})

	t.Run("propagates AppendCancelledAttempt error", func(t *testing.T) {
		store := newMemStore()
		store.errAppendCancelledAttempt = errors.New("append failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1__repo-a")
		require.ErrorContains(t, err, "append failed")
	})

	t.Run("propagates DeleteCandidateRun error", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		store.errDeleteCandidateRun = errors.New("delete failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1__repo-a")
		require.ErrorContains(t, err, "delete failed")
	})
}

func TestService_DryRun(t *testing.T) {
	ctx := context.Background()

	t.Run("passes migration steps to dry runner and returns result", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(ctx, api.RegisteredMigration{
			Id:    "m1",
			Steps: []api.StepDefinition{{Name: "step-1", WorkerApp: "app"}},
		})
		dr := &stubDryRunner{
			result: &api.DryRunResult{Steps: []api.StepDryRunResult{{StepName: "step-1"}}},
		}
		svc := newSvc(store, &stubEngine{}, dr)

		result, err := svc.DryRun(ctx, "m1", api.Candidate{Id: "repo-a"})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Steps, 1)
		// verify the runner received the correct steps and candidate
		assert.Len(t, dr.lastReq.Steps, 1)
		assert.Equal(t, "repo-a", dr.lastReq.Candidate.Id)
		assert.Equal(t, "m1", dr.lastReq.MigrationId)
	})

	t.Run("returns error when migration not found", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		_, err := svc.DryRun(ctx, "missing", api.Candidate{Id: "repo-a"})
		require.ErrorContains(t, err, "not found")
	})

	t.Run("propagates dry runner error", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(ctx, api.RegisteredMigration{Id: "m1"})
		dr := &stubDryRunner{err: errors.New("worker unreachable")}
		svc := newSvc(store, &stubEngine{}, dr)

		_, err := svc.DryRun(ctx, "m1", api.Candidate{Id: "repo-a"})
		require.ErrorContains(t, err, "worker unreachable")
	})

	t.Run("propagates store Get error", func(t *testing.T) {
		store := newMemStore()
		store.errGet = errors.New("store unavailable")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.DryRun(ctx, "m1", api.Candidate{Id: "repo-a"})
		require.ErrorContains(t, err, "store unavailable")
	})
}

func TestService_Execute(t *testing.T) {
	ctx := context.Background()

	saveMigration := func(store *memStore, runs map[string]api.CandidateRun) {
		var runsPtr *map[string]api.CandidateRun
		if runs != nil {
			runsPtr = &runs
		}
		_ = store.Save(ctx, api.RegisteredMigration{
			Id:            "m1",
			Steps:         []api.StepDefinition{{Name: "step-1"}},
			CandidateRuns: runsPtr,
		})
	}

	t.Run("starts workflow and marks candidate as running", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, nil)
		var startedID string
		engine := &stubEngine{
			startFn: func(_ context.Context, _, id string, _ any) (string, error) {
				startedID = id
				return id, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		runID, err := svc.Execute(ctx, "m1", api.Candidate{Id: "repo-a"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "m1__repo-a", runID)
		assert.Equal(t, "m1__repo-a", startedID)

		m, _ := store.Get(ctx, "m1")
		require.NotNil(t, m.CandidateRuns)
		assert.Equal(t, api.CandidateRunStatusRunning, (*m.CandidateRuns)["repo-a"].Status)
	})

	t.Run("merges inputs into candidate metadata before starting workflow", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, nil)
		var capturedManifest api.MigrationManifest
		engine := &stubEngine{
			startFn: func(_ context.Context, _, _ string, input any) (string, error) {
				b, _ := json.Marshal(input)
				_ = json.Unmarshal(b, &capturedManifest)
				return "id", nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Execute(ctx, "m1", api.Candidate{Id: "repo-a"}, map[string]string{"repoName": "my-repo"})
		require.NoError(t, err)
		require.NotNil(t, capturedManifest.Candidates[0].Metadata)
		assert.Equal(t, "my-repo", (*capturedManifest.Candidates[0].Metadata)["repoName"])
	})

	t.Run("blocks when candidate is already running and workflow still exists", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, map[string]api.CandidateRun{
			"repo-a": {Status: api.CandidateRunStatusRunning},
		})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
				return &migrations.WorkflowStatus{RuntimeStatus: "RUNNING"}, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Execute(ctx, "m1", api.Candidate{Id: "repo-a"}, nil)
		var alreadyRun migrations.CandidateAlreadyRunError
		require.ErrorAs(t, err, &alreadyRun)
		assert.Equal(t, "repo-a", alreadyRun.ID)
		assert.Equal(t, "running", alreadyRun.Status)
	})

	t.Run("blocks when candidate is completed and workflow still exists", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, map[string]api.CandidateRun{
			"repo-a": {Status: api.CandidateRunStatusCompleted},
		})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
				return &migrations.WorkflowStatus{RuntimeStatus: "COMPLETED"}, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Execute(ctx, "m1", api.Candidate{Id: "repo-a"}, nil)
		var alreadyRun migrations.CandidateAlreadyRunError
		require.ErrorAs(t, err, &alreadyRun)
		assert.Equal(t, "completed", alreadyRun.Status)
	})

	t.Run("allows re-execution when workflow is gone", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, map[string]api.CandidateRun{
			"repo-a": {Status: api.CandidateRunStatusRunning},
		})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, id string) (*migrations.WorkflowStatus, error) {
				return nil, migrations.WorkflowNotFoundError{InstanceID: id}
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		runID, err := svc.Execute(ctx, "m1", api.Candidate{Id: "repo-a"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "m1__repo-a", runID)
	})

	t.Run("returns error when migration not found", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		_, err := svc.Execute(ctx, "missing", api.Candidate{Id: "repo-a"}, nil)
		require.ErrorContains(t, err, "not found")
	})

	t.Run("propagates workflow start error", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, nil)
		engine := &stubEngine{
			startFn: func(_ context.Context, _, _ string, _ any) (string, error) {
				return "", errors.New("temporal down")
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Execute(ctx, "m1", api.Candidate{Id: "repo-a"}, nil)
		require.ErrorContains(t, err, "temporal down")
	})

	t.Run("propagates SetCandidateRun error", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, nil)
		store.errSetCandidateRun = errors.New("state write failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.Execute(ctx, "m1", api.Candidate{Id: "repo-a"}, nil)
		require.ErrorContains(t, err, "state write failed")
	})
}

func TestService_Start(t *testing.T) {
	t.Run("delegates to engine with MigrationOrchestrator workflow name", func(t *testing.T) {
		var capturedName string
		engine := &stubEngine{
			startFn: func(_ context.Context, name, id string, _ any) (string, error) {
				capturedName = name
				return id, nil
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		id, err := svc.Start(context.Background(), api.MigrationManifest{MigrationId: "run-123"})
		require.NoError(t, err)
		assert.Equal(t, "run-123", id)
		assert.Equal(t, "MigrationOrchestrator", capturedName)
	})

	t.Run("propagates engine error", func(t *testing.T) {
		engine := &stubEngine{
			startFn: func(_ context.Context, _, _ string, _ any) (string, error) {
				return "", errors.New("temporal unavailable")
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		_, err := svc.Start(context.Background(), api.MigrationManifest{})
		require.ErrorContains(t, err, "temporal unavailable")
	})
}

func TestService_Status(t *testing.T) {
	ctx := context.Background()

	t.Run("returns status with parsed result when output is present", func(t *testing.T) {
		result := api.MigrationResult{MigrationId: "run-123", Status: "completed"}
		output, _ := json.Marshal(result)
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
				return &migrations.WorkflowStatus{RuntimeStatus: "COMPLETED", Output: output}, nil
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		ms, err := svc.Status(ctx, "run-123")
		require.NoError(t, err)
		assert.Equal(t, "COMPLETED", ms.RuntimeStatus)
		assert.Equal(t, "run-123", ms.InstanceID)
		require.NotNil(t, ms.Result)
		assert.Equal(t, "run-123", ms.Result.MigrationId)
	})

	t.Run("returns status without result when no output", func(t *testing.T) {
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
				return &migrations.WorkflowStatus{RuntimeStatus: "RUNNING"}, nil
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		ms, err := svc.Status(ctx, "run-123")
		require.NoError(t, err)
		assert.Equal(t, "RUNNING", ms.RuntimeStatus)
		assert.Nil(t, ms.Result)
	})

	t.Run("propagates engine error", func(t *testing.T) {
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.WorkflowStatus, error) {
				return nil, errors.New("engine down")
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		_, err := svc.Status(ctx, "run-123")
		require.ErrorContains(t, err, "engine down")
	})
}

func TestService_HandleEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("raises event with deterministic step-completed name", func(t *testing.T) {
		var raisedEvent string
		engine := &stubEngine{
			raiseEventFn: func(_ context.Context, _, event string, _ any) error {
				raisedEvent = event
				return nil
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})
		candidate := api.Candidate{Id: "repo-a"}

		err := svc.HandleEvent(ctx, "run-123", api.StepCompletedEvent{StepName: "step-1", Candidate: candidate})
		require.NoError(t, err)
		assert.Equal(t, migrations.StepEventName("step-1", candidate), raisedEvent)
	})

	t.Run("propagates engine error", func(t *testing.T) {
		engine := &stubEngine{
			raiseEventFn: func(_ context.Context, _, _ string, _ any) error {
				return errors.New("signal failed")
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		err := svc.HandleEvent(ctx, "run-123", api.StepCompletedEvent{})
		require.ErrorContains(t, err, "signal failed")
	})
}

func TestService_HandlePROpened(t *testing.T) {
	ctx := context.Background()

	t.Run("raises event with deterministic pr-opened name", func(t *testing.T) {
		var raisedEvent string
		engine := &stubEngine{
			raiseEventFn: func(_ context.Context, _, event string, _ any) error {
				raisedEvent = event
				return nil
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})
		candidate := api.Candidate{Id: "repo-a"}

		err := svc.HandlePROpened(ctx, "run-123", api.StepCompletedEvent{StepName: "step-1", Candidate: candidate})
		require.NoError(t, err)
		assert.Equal(t, migrations.PROpenedEventName("step-1", candidate), raisedEvent)
	})

	t.Run("propagates engine error", func(t *testing.T) {
		engine := &stubEngine{
			raiseEventFn: func(_ context.Context, _, _ string, _ any) error {
				return errors.New("signal failed")
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		err := svc.HandlePROpened(ctx, "run-123", api.StepCompletedEvent{})
		require.ErrorContains(t, err, "signal failed")
	})
}
