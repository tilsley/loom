package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations/store"
	"github.com/tilsley/loom/pkg/api"
)

// newStore starts a miniredis server and returns a RedisMigrationStore backed by it.
// The server is stopped automatically when the test ends.
func newStore(t *testing.T) *store.RedisMigrationStore {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return store.NewRedisMigrationStore(rdb)
}

var baseMigration = api.Migration{
	Id:          "app-chart-migration",
	Name:        "App Chart Migration",
	Description: "Migrate all app charts",
	CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	MigratorUrl: "http://app-chart-migrator:3001",
	Steps:       []api.StepDefinition{{Name: "update-chart", MigratorApp: "app-chart-migrator"}},
}

// ─── Save / Get roundtrip ────────────────────────────────────────────────────

func TestSaveGet_NoCandidates(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = nil

	require.NoError(t, s.Save(context.Background(), m))

	got, err := s.Get(context.Background(), m.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, m.Id, got.Id)
	assert.Empty(t, got.Candidates)
}

func TestSaveGet_WithCandidates(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application"},
		{Id: "payments-api", Kind: "application"},
	}

	require.NoError(t, s.Save(context.Background(), m))

	got, err := s.Get(context.Background(), m.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Candidates, 2)
	ids := []string{got.Candidates[0].Id, got.Candidates[1].Id}
	assert.ElementsMatch(t, []string{"billing-api", "payments-api"}, ids)
}

func TestGet_NotFound_ReturnsNil(t *testing.T) {
	s := newStore(t)

	got, err := s.Get(context.Background(), "nonexistent")

	require.NoError(t, err)
	assert.Nil(t, got)
}

// CandidatesField verifies the migration JSON stored in Redis has nil candidates
// (so candidates are never duplicated between the migration blob and individual keys).
func TestSave_MigrationBlobHasNilCandidates(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	s := store.NewRedisMigrationStore(rdb)

	m := baseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	// Read the raw migration blob directly from Redis.
	raw, err := rdb.Get(context.Background(), "migration:app-chart-migration").Result()
	require.NoError(t, err)
	assert.NotContains(t, raw, `"billing-api"`,
		"candidate data must not be embedded in the migration JSON blob")
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestList_Empty(t *testing.T) {
	s := newStore(t)

	migrations, err := s.List(context.Background())

	require.NoError(t, err)
	assert.Empty(t, migrations)
}

func TestList_ReturnsSavedMigrations(t *testing.T) {
	s := newStore(t)
	m1 := baseMigration
	m2 := baseMigration
	m2.Id = "another-migration"
	require.NoError(t, s.Save(context.Background(), m1))
	require.NoError(t, s.Save(context.Background(), m2))

	migrations, err := s.List(context.Background())

	require.NoError(t, err)
	require.Len(t, migrations, 2)
	ids := []string{migrations[0].Id, migrations[1].Id}
	assert.ElementsMatch(t, []string{m1.Id, m2.Id}, ids)
}

func TestList_IncludesCandidates(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	migrations, err := s.List(context.Background())
	require.NoError(t, err)
	require.Len(t, migrations, 1)
	assert.Len(t, migrations[0].Candidates, 1)
	assert.Equal(t, "billing-api", migrations[0].Candidates[0].Id)
}

// ─── SetCandidateStatus ───────────────────────────────────────────────────────

func TestSetCandidateStatus_UpdatesStatus(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	err := s.SetCandidateStatus(context.Background(), m.Id, "billing-api", api.CandidateStatusRunning)
	require.NoError(t, err)

	got, err := s.Get(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, got.Candidates, 1)
	assert.Equal(t, api.CandidateStatusRunning, got.Candidates[0].Status)
}

func TestSetCandidateStatus_DoesNotTouchMigrationKey(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	s := store.NewRedisMigrationStore(rdb)

	m := baseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	// Snapshot the migration blob before the status update.
	before, err := rdb.Get(context.Background(), "migration:app-chart-migration").Result()
	require.NoError(t, err)

	require.NoError(t, s.SetCandidateStatus(context.Background(), m.Id, "billing-api", api.CandidateStatusRunning))

	after, err := rdb.Get(context.Background(), "migration:app-chart-migration").Result()
	require.NoError(t, err)
	assert.Equal(t, before, after, "SetCandidateStatus must not rewrite the migration key")
}

func TestSetCandidateStatus_CandidateNotFound(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	err := s.SetCandidateStatus(context.Background(), m.Id, "nonexistent-candidate", api.CandidateStatusRunning)

	assert.Error(t, err)
}

// ─── SaveCandidates ───────────────────────────────────────────────────────────

func TestSaveCandidates_SetsNotStartedStatus(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = nil
	require.NoError(t, s.Save(context.Background(), m))

	incoming := []api.Candidate{
		{Id: "billing-api", Kind: "application"},
		{Id: "payments-api", Kind: "application"},
	}
	require.NoError(t, s.SaveCandidates(context.Background(), m.Id, incoming))

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	for _, c := range candidates {
		assert.Equal(t, api.CandidateStatusNotStarted, c.Status, "candidate %q should be not_started", c.Id)
	}
}

func TestSaveCandidates_PreservesRunningCandidate(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application", Status: api.CandidateStatusRunning},
	}
	require.NoError(t, s.Save(context.Background(), m))

	// Re-submit the same candidate list (as a discoverer would on re-announce).
	incoming := []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.SaveCandidates(context.Background(), m.Id, incoming))

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, api.CandidateStatusRunning, candidates[0].Status,
		"running candidate status must be preserved")
}

func TestSaveCandidates_PreservesCompletedCandidate(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application", Status: api.CandidateStatusCompleted},
	}
	require.NoError(t, s.Save(context.Background(), m))

	incoming := []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.SaveCandidates(context.Background(), m.Id, incoming))

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, api.CandidateStatusCompleted, candidates[0].Status)
}

func TestSaveCandidates_KeepsRunningCandidateRemovedFromIncoming(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application", Status: api.CandidateStatusRunning},
	}
	require.NoError(t, s.Save(context.Background(), m))

	// Incoming list no longer contains billing-api (e.g. discoverer filtered it).
	incoming := []api.Candidate{{Id: "payments-api", Kind: "application"}}
	require.NoError(t, s.SaveCandidates(context.Background(), m.Id, incoming))

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.Id
	}
	assert.Contains(t, ids, "billing-api", "running candidate removed from incoming list must be retained")
}

func TestSaveCandidates_MigrationNotFound(t *testing.T) {
	s := newStore(t)

	err := s.SaveCandidates(context.Background(), "nonexistent", []api.Candidate{{Id: "billing-api"}})

	assert.Error(t, err)
}

// ─── GetCandidates ────────────────────────────────────────────────────────────

func TestGetCandidates_ReturnsEmpty_WhenNoneStored(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = nil
	require.NoError(t, s.Save(context.Background(), m))

	candidates, err := s.GetCandidates(context.Background(), m.Id)

	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestGetCandidates_ReturnsAllCandidates(t *testing.T) {
	s := newStore(t)
	m := baseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application"},
		{Id: "payments-api", Kind: "application"},
		{Id: "auth-service", Kind: "application"},
	}
	require.NoError(t, s.Save(context.Background(), m))

	candidates, err := s.GetCandidates(context.Background(), m.Id)

	require.NoError(t, err)
	require.Len(t, candidates, 3)
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.Id
	}
	assert.ElementsMatch(t, []string{"billing-api", "payments-api", "auth-service"}, ids)
}
