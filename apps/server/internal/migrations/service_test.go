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
	_ migrations.ExecutionEngine = (*stubEngine)(nil)
	_ migrations.DryRunner      = (*stubDryRunner)(nil)
)

func ptr[T any](v T) *T { return &v }

// ─── stubEngine ───────────────────────────────────────────────────────────────

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

// ─── stubDryRunner ────────────────────────────────────────────────────────────

type stubDryRunner struct {
	result          *api.DryRunResult
	err             error
	lastReq         api.DryRunRequest
	lastWorkerUrl   string
}

func (d *stubDryRunner) DryRun(_ context.Context, workerUrl string, req api.DryRunRequest) (*api.DryRunResult, error) {
	d.lastWorkerUrl = workerUrl
	d.lastReq = req
	return d.result, d.err
}

// ─── memStore ─────────────────────────────────────────────────────────────────

type memStore struct {
	data map[string]api.Migration

	// per-method error stubs
	errSave               error
	errGet                error
	errList               error
	errSetCandidateStatus error
	errSaveCandidates     error
	errGetCandidates      error
}

func newMemStore() *memStore {
	return &memStore{
		data: make(map[string]api.Migration),
	}
}

func (s *memStore) Save(_ context.Context, m api.Migration) error {
	if s.errSave != nil {
		return s.errSave
	}
	s.data[m.Id] = m
	return nil
}

func (s *memStore) Get(_ context.Context, id string) (*api.Migration, error) {
	if s.errGet != nil {
		return nil, s.errGet
	}
	m, ok := s.data[id]
	if !ok {
		return nil, nil
	}
	return &m, nil
}

func (s *memStore) List(_ context.Context) ([]api.Migration, error) {
	if s.errList != nil {
		return nil, s.errList
	}
	out := make([]api.Migration, 0, len(s.data))
	for _, m := range s.data {
		out = append(out, m)
	}
	return out, nil
}

func (s *memStore) SetCandidateStatus(_ context.Context, migrationID, candidateID string, status api.CandidateStatus) error {
	if s.errSetCandidateStatus != nil {
		return s.errSetCandidateStatus
	}
	m, ok := s.data[migrationID]
	if !ok {
		return nil
	}
	for i, c := range m.Candidates {
		if c.Id == candidateID {
			st := status
			m.Candidates[i].Status = &st
			s.data[migrationID] = m
			return nil
		}
	}
	return nil
}

func (s *memStore) SaveCandidates(_ context.Context, migrationID string, candidates []api.Candidate) error {
	if s.errSaveCandidates != nil {
		return s.errSaveCandidates
	}
	m, ok := s.data[migrationID]
	if !ok {
		return fmt.Errorf("migration %q not found", migrationID)
	}
	ns := api.CandidateStatusNotStarted
	for i := range candidates {
		if candidates[i].Status == nil {
			candidates[i].Status = &ns
		}
	}
	m.Candidates = candidates
	s.data[migrationID] = m
	return nil
}

func (s *memStore) GetCandidates(_ context.Context, migrationID string) ([]api.Candidate, error) {
	if s.errGetCandidates != nil {
		return nil, s.errGetCandidates
	}
	m, ok := s.data[migrationID]
	if !ok {
		return nil, nil
	}
	return m.Candidates, nil
}

// ─── constructor helper ───────────────────────────────────────────────────────

func newSvc(store *memStore, engine *stubEngine, dr *stubDryRunner) *migrations.Service {
	return migrations.NewService(engine, store, dr)
}

// ─── tests ────────────────────────────────────────────────────────────────────

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
		assert.WithinDuration(t, time.Now(), m.CreatedAt, 2*time.Second)
	})

	t.Run("upserts existing migration preserving createdAt and candidates", func(t *testing.T) {
		store := newMemStore()
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})
		ctx := context.Background()

		createdAt := time.Now().Add(-24 * time.Hour).UTC()
		_ = store.Save(ctx, api.Migration{
			Id:         "app-chart-migration",
			Name:       "Old Name",
			CreatedAt:  createdAt,
			Candidates: []api.Candidate{{Id: "repo-a"}},
		})

		m, err := svc.Announce(ctx, api.MigrationAnnouncement{
			Id:    "app-chart-migration",
			Name:  "App Chart Migration",
			Steps: []api.StepDefinition{{Name: "step-1", MigratorApp: "app"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "App Chart Migration", m.Name)
		assert.Equal(t, createdAt, m.CreatedAt, "createdAt must be preserved")
		assert.Len(t, m.Candidates, 1, "candidates must be preserved")
		assert.Len(t, m.Steps, 1, "steps must be updated")
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
		_ = store.Save(context.Background(), api.Migration{Id: "m1", Name: "M1"})
		_ = store.Save(context.Background(), api.Migration{Id: "m2", Name: "M2"})
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
		_ = store.Save(context.Background(), api.Migration{Id: "m1", Name: "M1"})
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

func TestService_SubmitCandidates(t *testing.T) {
	t.Run("saves candidates for known migration", func(t *testing.T) {
		store := newMemStore()
		ctx := context.Background()
		_ = store.Save(ctx, api.Migration{Id: "m1"})
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
	t.Run("returns candidates unchanged when none are running", func(t *testing.T) {
		store := newMemStore()
		ns := api.CandidateStatusNotStarted
		completed := api.CandidateStatusCompleted
		_ = store.Save(context.Background(), api.Migration{
			Id: "m1",
			Candidates: []api.Candidate{
				{Id: "repo-a", Status: &ns},
				{Id: "repo-b", Status: &completed},
			},
		})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		assert.Len(t, cs, 2)
		require.NotNil(t, cs[0].Status)
		assert.Equal(t, api.CandidateStatusNotStarted, *cs[0].Status)
	})

	t.Run("running candidate with live run is unchanged", func(t *testing.T) {
		store := newMemStore()
		running := api.CandidateStatusRunning
		_ = store.Save(context.Background(), api.Migration{
			Id:         "m1",
			Candidates: []api.Candidate{{Id: "repo-a", Status: &running}},
		})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.RunStatus, error) {
				return &migrations.RunStatus{RuntimeStatus: "RUNNING"}, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		require.NotNil(t, cs[0].Status)
		assert.Equal(t, api.CandidateStatusRunning, *cs[0].Status)
	})

	t.Run("stale running candidate is reset to not_started", func(t *testing.T) {
		store := newMemStore()
		running := api.CandidateStatusRunning
		_ = store.Save(context.Background(), api.Migration{
			Id:         "m1",
			Candidates: []api.Candidate{{Id: "repo-a", Status: &running}},
		})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, id string) (*migrations.RunStatus, error) {
				return nil, migrations.RunNotFoundError{InstanceID: id}
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		require.NotNil(t, cs[0].Status)
		assert.Equal(t, api.CandidateStatusNotStarted, *cs[0].Status)
	})

	t.Run("non-not-found engine error leaves candidate unchanged", func(t *testing.T) {
		store := newMemStore()
		running := api.CandidateStatusRunning
		_ = store.Save(context.Background(), api.Migration{
			Id:         "m1",
			Candidates: []api.Candidate{{Id: "repo-a", Status: &running}},
		})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.RunStatus, error) {
				return nil, errors.New("connection timeout")
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		cs, err := svc.GetCandidates(context.Background(), "m1")
		require.NoError(t, err)
		require.NotNil(t, cs[0].Status)
		assert.Equal(t, api.CandidateStatusRunning, *cs[0].Status, "should not reset on non-not-found error")
	})

	t.Run("propagates store error", func(t *testing.T) {
		store := newMemStore()
		store.errGetCandidates = errors.New("store unavailable")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.GetCandidates(context.Background(), "m1")
		require.ErrorContains(t, err, "store unavailable")
	})
}

func TestService_GetCandidateSteps(t *testing.T) {
	ctx := context.Background()

	t.Run("returns completed steps when run output is present", func(t *testing.T) {
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.RunStatus, error) {
				return &migrations.RunStatus{
					RuntimeStatus: "COMPLETED",
					Steps:         []api.StepState{{StepName: "step-1", Candidate: api.Candidate{Id: "repo-a"}, Status: api.StepStateStatusSucceeded}},
				}, nil
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		resp, err := svc.GetCandidateSteps(ctx, "m1", "repo-a")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, api.CandidateStepsResponseStatusCompleted, resp.Status)
		assert.Len(t, resp.Steps, 1)
		assert.Equal(t, "step-1", resp.Steps[0].StepName)
	})

	t.Run("returns running status when run has no output yet", func(t *testing.T) {
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.RunStatus, error) {
				return &migrations.RunStatus{RuntimeStatus: "RUNNING"}, nil
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		resp, err := svc.GetCandidateSteps(ctx, "m1", "repo-a")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, api.CandidateStepsResponseStatusRunning, resp.Status)
		assert.Empty(t, resp.Steps)
	})

	t.Run("returns nil when run not found", func(t *testing.T) {
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, id string) (*migrations.RunStatus, error) {
				return nil, migrations.RunNotFoundError{InstanceID: id}
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		resp, err := svc.GetCandidateSteps(ctx, "m1", "repo-a")
		require.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("propagates engine error", func(t *testing.T) {
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.RunStatus, error) {
				return nil, errors.New("engine down")
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		_, err := svc.GetCandidateSteps(ctx, "m1", "repo-a")
		require.ErrorContains(t, err, "engine down")
	})
}

func TestService_RetryStep(t *testing.T) {
	ctx := context.Background()
	running := api.CandidateStatusRunning

	setup := func(store *memStore) {
		_ = store.Save(ctx, api.Migration{
			Id:         "m1",
			Candidates: []api.Candidate{{Id: "repo-a", Status: &running}},
		})
	}

	t.Run("raises retry signal for running candidate", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		var raisedEvent string
		engine := &stubEngine{
			raiseEventFn: func(_ context.Context, _, event string, _ any) error {
				raisedEvent = event
				return nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		err := svc.RetryStep(ctx, "m1", "repo-a", "step-1")
		require.NoError(t, err)
		assert.Equal(t, migrations.RetryStepEventName("step-1", "repo-a"), raisedEvent)
	})

	t.Run("migration not found returns error", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		err := svc.RetryStep(ctx, "unknown", "repo-a", "step-1")
		require.ErrorContains(t, err, "not found")
	})

	t.Run("candidate not found returns error", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(ctx, api.Migration{Id: "m1", Candidates: []api.Candidate{{Id: "other"}}})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.RetryStep(ctx, "m1", "repo-a", "step-1")
		require.ErrorContains(t, err, "not found")
	})

	t.Run("candidate not running returns CandidateNotRunningError", func(t *testing.T) {
		store := newMemStore()
		notStarted := api.CandidateStatusNotStarted
		_ = store.Save(ctx, api.Migration{
			Id:         "m1",
			Candidates: []api.Candidate{{Id: "repo-a", Status: &notStarted}},
		})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.RetryStep(ctx, "m1", "repo-a", "step-1")
		var notRunning migrations.CandidateNotRunningError
		require.ErrorAs(t, err, &notRunning)
		assert.Equal(t, "repo-a", notRunning.ID)
	})

	t.Run("propagates engine error", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		engine := &stubEngine{
			raiseEventFn: func(_ context.Context, _, _ string, _ any) error {
				return errors.New("signal failed")
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		err := svc.RetryStep(ctx, "m1", "repo-a", "step-1")
		require.ErrorContains(t, err, "signal failed")
	})
}

func TestService_Cancel(t *testing.T) {
	ctx := context.Background()
	running := api.CandidateStatusRunning

	setup := func(store *memStore) {
		_ = store.Save(ctx, api.Migration{
			Id:         "m1",
			Candidates: []api.Candidate{{Id: "repo-a", Status: &running}},
		})
	}

	t.Run("cancels run and resets candidate to not_started", func(t *testing.T) {
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

		err := svc.Cancel(ctx, "m1", "repo-a")
		require.NoError(t, err)
		assert.Equal(t, "m1__repo-a", cancelledID)
		m, _ := store.Get(ctx, "m1")
		require.NotNil(t, m.Candidates[0].Status)
		assert.Equal(t, api.CandidateStatusNotStarted, *m.Candidates[0].Status)
	})

	t.Run("run not found is tolerated; reset still proceeds", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		engine := &stubEngine{
			cancelFn: func(_ context.Context, id string) error {
				return migrations.RunNotFoundError{InstanceID: id}
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1", "repo-a")
		require.NoError(t, err)
		m, _ := store.Get(ctx, "m1")
		require.NotNil(t, m.Candidates[0].Status)
		assert.Equal(t, api.CandidateStatusNotStarted, *m.Candidates[0].Status)
	})

	t.Run("other engine error is returned immediately", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		engine := &stubEngine{
			cancelFn: func(_ context.Context, _ string) error {
				return errors.New("temporal unavailable")
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1", "repo-a")
		require.ErrorContains(t, err, "temporal unavailable")
	})

	t.Run("propagates SetCandidateStatus error", func(t *testing.T) {
		store := newMemStore()
		setup(store)
		store.errSetCandidateStatus = errors.New("delete failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1", "repo-a")
		require.ErrorContains(t, err, "delete failed")
	})

	t.Run("migration not found returns error", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		err := svc.Cancel(ctx, "unknown", "repo-a")
		require.ErrorContains(t, err, "not found")
	})

	t.Run("candidate not found returns error", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(ctx, api.Migration{Id: "m1", Candidates: []api.Candidate{{Id: "other"}}})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1", "repo-a")
		require.ErrorContains(t, err, "not found")
	})

	t.Run("candidate not running returns CandidateNotRunningError", func(t *testing.T) {
		store := newMemStore()
		notStarted := api.CandidateStatusNotStarted
		_ = store.Save(ctx, api.Migration{
			Id:         "m1",
			Candidates: []api.Candidate{{Id: "repo-a", Status: &notStarted}},
		})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		err := svc.Cancel(ctx, "m1", "repo-a")
		var notRunning migrations.CandidateNotRunningError
		require.ErrorAs(t, err, &notRunning)
		assert.Equal(t, "repo-a", notRunning.ID)
	})
}

func TestService_DryRun(t *testing.T) {
	ctx := context.Background()

	t.Run("passes migration steps to dry runner and returns result", func(t *testing.T) {
		store := newMemStore()
		_ = store.Save(ctx, api.Migration{
			Id:    "m1",
			Steps: []api.StepDefinition{{Name: "step-1", MigratorApp: "app"}},
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
		_ = store.Save(ctx, api.Migration{Id: "m1"})
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

func TestService_Start(t *testing.T) {
	ctx := context.Background()

	// saveMigration saves m1 with the given candidates pre-populated.
	saveMigration := func(store *memStore, candidates []api.Candidate) {
		_ = store.Save(ctx, api.Migration{
			Id:         "m1",
			Steps:      []api.StepDefinition{{Name: "step-1"}},
			Candidates: candidates,
		})
	}

	t.Run("starts run and marks candidate as running", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, []api.Candidate{{Id: "repo-a"}})
		var startedID string
		engine := &stubEngine{
			startFn: func(_ context.Context, _, id string, _ any) (string, error) {
				startedID = id
				return id, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		runID, err := svc.Start(ctx, "m1", "repo-a", nil)
		require.NoError(t, err)
		assert.Equal(t, "m1__repo-a", runID)
		assert.Equal(t, "m1__repo-a", startedID)

		m, _ := store.Get(ctx, "m1")
		require.Len(t, m.Candidates, 1)
		require.NotNil(t, m.Candidates[0].Status)
		assert.Equal(t, api.CandidateStatusRunning, *m.Candidates[0].Status)
	})

	t.Run("merges inputs into candidate metadata before starting run", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, []api.Candidate{{Id: "repo-a"}})
		var capturedManifest api.MigrationManifest
		engine := &stubEngine{
			startFn: func(_ context.Context, _, _ string, input any) (string, error) {
				b, _ := json.Marshal(input)
				_ = json.Unmarshal(b, &capturedManifest)
				return "id", nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Start(ctx, "m1", "repo-a", map[string]string{"repoName": "my-repo"})
		require.NoError(t, err)
		require.NotNil(t, capturedManifest.Candidates[0].Metadata)
		assert.Equal(t, "my-repo", (*capturedManifest.Candidates[0].Metadata)["repoName"])
	})

	t.Run("manifest MigrationId is the migration ID, not the run ID", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, []api.Candidate{{Id: "repo-a"}})
		var capturedManifest api.MigrationManifest
		engine := &stubEngine{
			startFn: func(_ context.Context, _, _ string, input any) (string, error) {
				b, _ := json.Marshal(input)
				_ = json.Unmarshal(b, &capturedManifest)
				return "id", nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Start(ctx, "m1", "repo-a", nil)
		require.NoError(t, err)
		assert.Equal(t, "m1", capturedManifest.MigrationId)
	})

	t.Run("blocks when candidate is already running and run still exists", func(t *testing.T) {
		store := newMemStore()
		running := api.CandidateStatusRunning
		saveMigration(store, []api.Candidate{{Id: "repo-a", Status: &running}})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.RunStatus, error) {
				return &migrations.RunStatus{RuntimeStatus: "RUNNING"}, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Start(ctx, "m1", "repo-a", nil)
		var alreadyRun migrations.CandidateAlreadyRunError
		require.ErrorAs(t, err, &alreadyRun)
		assert.Equal(t, "repo-a", alreadyRun.ID)
		assert.Equal(t, "running", alreadyRun.Status)
	})

	t.Run("blocks when candidate is completed and run still exists", func(t *testing.T) {
		store := newMemStore()
		completed := api.CandidateStatusCompleted
		saveMigration(store, []api.Candidate{{Id: "repo-a", Status: &completed}})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, _ string) (*migrations.RunStatus, error) {
				return &migrations.RunStatus{RuntimeStatus: "COMPLETED"}, nil
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Start(ctx, "m1", "repo-a", nil)
		var alreadyRun migrations.CandidateAlreadyRunError
		require.ErrorAs(t, err, &alreadyRun)
		assert.Equal(t, "completed", alreadyRun.Status)
	})

	t.Run("allows re-execution when run is gone", func(t *testing.T) {
		store := newMemStore()
		running := api.CandidateStatusRunning
		saveMigration(store, []api.Candidate{{Id: "repo-a", Status: &running}})
		engine := &stubEngine{
			getStatusFn: func(_ context.Context, id string) (*migrations.RunStatus, error) {
				return nil, migrations.RunNotFoundError{InstanceID: id}
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		runID, err := svc.Start(ctx, "m1", "repo-a", nil)
		require.NoError(t, err)
		assert.Equal(t, "m1__repo-a", runID)
	})

	t.Run("returns error when migration not found", func(t *testing.T) {
		svc := newSvc(newMemStore(), &stubEngine{}, &stubDryRunner{})

		_, err := svc.Start(ctx, "missing", "repo-a", nil)
		require.ErrorContains(t, err, "not found")
	})

	t.Run("returns error when candidate not found in migration", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, []api.Candidate{{Id: "other-repo"}})
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.Start(ctx, "m1", "repo-a", nil)
		require.ErrorContains(t, err, "candidate")
		require.ErrorContains(t, err, "not found")
	})

	t.Run("propagates run start error", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, []api.Candidate{{Id: "repo-a"}})
		engine := &stubEngine{
			startFn: func(_ context.Context, _, _ string, _ any) (string, error) {
				return "", errors.New("temporal down")
			},
		}
		svc := newSvc(store, engine, &stubDryRunner{})

		_, err := svc.Start(ctx, "m1", "repo-a", nil)
		require.ErrorContains(t, err, "temporal down")
	})

	t.Run("propagates SetCandidateStatus error", func(t *testing.T) {
		store := newMemStore()
		saveMigration(store, []api.Candidate{{Id: "repo-a"}})
		store.errSetCandidateStatus = errors.New("state write failed")
		svc := newSvc(store, &stubEngine{}, &stubDryRunner{})

		_, err := svc.Start(ctx, "m1", "repo-a", nil)
		require.ErrorContains(t, err, "state write failed")
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

		err := svc.HandleEvent(ctx, "run-123", api.StepStatusEvent{StepName: "step-1", CandidateId: candidate.Id})
		require.NoError(t, err)
		assert.Equal(t, migrations.StepEventName("step-1", candidate.Id), raisedEvent)
	})

	t.Run("propagates engine error", func(t *testing.T) {
		engine := &stubEngine{
			raiseEventFn: func(_ context.Context, _, _ string, _ any) error {
				return errors.New("signal failed")
			},
		}
		svc := newSvc(newMemStore(), engine, &stubDryRunner{})

		err := svc.HandleEvent(ctx, "run-123", api.StepStatusEvent{})
		require.ErrorContains(t, err, "signal failed")
	})
}

// Ensure ptr is used (suppress unused warning)
var _ = ptr[string]
