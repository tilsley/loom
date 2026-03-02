package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilsley/loom/apps/server/internal/migrations/store"
	"github.com/tilsley/loom/apps/server/internal/migrations/store/pgmigrations"
	pgplatform "github.com/tilsley/loom/apps/server/internal/platform/postgres"
	"github.com/tilsley/loom/pkg/api"
)

// newPGStore creates a PGMigrationStore backed by a real PostgreSQL instance.
// Skips if POSTGRES_URL is not set.
func newPGStore(t *testing.T) *store.PGMigrationStore {
	t.Helper()
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		t.Skip("POSTGRES_URL not set — skipping Postgres integration tests")
	}
	pool, err := pgplatform.New(context.Background(), pgURL, pgmigrations.FS)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupPGStore(t, pool)
		pool.Close()
	})
	return store.NewPGMigrationStore(pool)
}

func cleanupPGStore(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `DELETE FROM candidates; DELETE FROM migrations;`)
	require.NoError(t, err)
}

var pgBaseMigration = api.Migration{
	Id:          "app-chart-migration",
	Name:        "App Chart Migration",
	Description: "Migrate all app charts",
	CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	MigratorUrl: "http://app-chart-migrator:3001",
	Steps:       []api.StepDefinition{{Name: "update-chart", MigratorApp: "app-chart-migrator"}},
}

// ─── Save / Get roundtrip ────────────────────────────────────────────────────

func TestPG_SaveGet_NoCandidates(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = nil

	require.NoError(t, s.Save(context.Background(), m))

	got, err := s.Get(context.Background(), m.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, m.Id, got.Id)
	assert.Equal(t, m.Name, got.Name)
	assert.Equal(t, m.MigratorUrl, got.MigratorUrl)
	assert.Empty(t, got.Candidates)
}

func TestPG_SaveGet_WithCandidates(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
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

func TestPG_Get_NotFound_ReturnsNil(t *testing.T) {
	s := newPGStore(t)

	got, err := s.Get(context.Background(), "nonexistent")

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPG_Save_Idempotent(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = nil

	require.NoError(t, s.Save(context.Background(), m))
	// Second save should not error (upsert).
	require.NoError(t, s.Save(context.Background(), m))

	got, err := s.Get(context.Background(), m.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestPG_List_Empty(t *testing.T) {
	s := newPGStore(t)

	migrations, err := s.List(context.Background())

	require.NoError(t, err)
	assert.Empty(t, migrations)
}

func TestPG_List_ReturnsSavedMigrations(t *testing.T) {
	s := newPGStore(t)
	m1 := pgBaseMigration
	m2 := pgBaseMigration
	m2.Id = "another-migration"
	require.NoError(t, s.Save(context.Background(), m1))
	require.NoError(t, s.Save(context.Background(), m2))

	migrations, err := s.List(context.Background())

	require.NoError(t, err)
	require.Len(t, migrations, 2)
	ids := []string{migrations[0].Id, migrations[1].Id}
	assert.ElementsMatch(t, []string{m1.Id, m2.Id}, ids)
}

func TestPG_List_IncludesCandidates(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	migrations, err := s.List(context.Background())
	require.NoError(t, err)
	require.Len(t, migrations, 1)
	assert.Len(t, migrations[0].Candidates, 1)
	assert.Equal(t, "billing-api", migrations[0].Candidates[0].Id)
}

// ─── SetCandidateStatus ───────────────────────────────────────────────────────

func TestPG_SetCandidateStatus_UpdatesStatus(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	err := s.SetCandidateStatus(context.Background(), m.Id, "billing-api", api.CandidateStatusRunning)
	require.NoError(t, err)

	got, err := s.Get(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, got.Candidates, 1)
	assert.Equal(t, api.CandidateStatusRunning, got.Candidates[0].Status)
}

func TestPG_SetCandidateStatus_CandidateNotFound(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	err := s.SetCandidateStatus(context.Background(), m.Id, "nonexistent-candidate", api.CandidateStatusRunning)

	assert.Error(t, err)
}

// ─── SaveCandidates ───────────────────────────────────────────────────────────

func TestPG_SaveCandidates_SetsNotStartedStatus(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
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

func TestPG_SaveCandidates_PreservesRunningCandidate(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application", Status: api.CandidateStatusRunning},
	}
	require.NoError(t, s.Save(context.Background(), m))

	incoming := []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.SaveCandidates(context.Background(), m.Id, incoming))

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, api.CandidateStatusRunning, candidates[0].Status,
		"running candidate status must be preserved")
}

func TestPG_SaveCandidates_PreservesCompletedCandidate(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
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

func TestPG_SaveCandidates_KeepsRunningCandidateRemovedFromIncoming(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application", Status: api.CandidateStatusRunning},
	}
	require.NoError(t, s.Save(context.Background(), m))

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

func TestPG_SaveCandidates_MigrationNotFound(t *testing.T) {
	s := newPGStore(t)

	err := s.SaveCandidates(context.Background(), "nonexistent", []api.Candidate{{Id: "billing-api"}})

	assert.Error(t, err)
}

func TestPG_SaveCandidates_MergesMetadata(t *testing.T) {
	s := newPGStore(t)
	existingMeta := map[string]string{"team": "platform", "env": "prod"}
	m := pgBaseMigration
	m.Candidates = []api.Candidate{
		{Id: "billing-api", Kind: "application", Metadata: &existingMeta},
	}
	require.NoError(t, s.Save(context.Background(), m))

	// Re-discover with new metadata — existing values should win.
	newMeta := map[string]string{"team": "new-team", "repoName": "billing-api"}
	incoming := []api.Candidate{{Id: "billing-api", Kind: "application", Metadata: &newMeta}}
	require.NoError(t, s.SaveCandidates(context.Background(), m.Id, incoming))

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].Metadata)
	// Existing "team" value wins.
	assert.Equal(t, "platform", (*candidates[0].Metadata)["team"])
	// New key is added.
	assert.Equal(t, "billing-api", (*candidates[0].Metadata)["repoName"])
}

// ─── GetCandidates ────────────────────────────────────────────────────────────

func TestPG_GetCandidates_ReturnsEmpty_WhenNoneStored(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = nil
	require.NoError(t, s.Save(context.Background(), m))

	candidates, err := s.GetCandidates(context.Background(), m.Id)

	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestPG_GetCandidates_ReturnsAllCandidates(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
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

// ─── UpdateCandidateMetadata ──────────────────────────────────────────────────

func TestPG_UpdateCandidateMetadata_CreatesIfNil(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application"}}
	require.NoError(t, s.Save(context.Background(), m))

	err := s.UpdateCandidateMetadata(context.Background(), m.Id, "billing-api",
		map[string]string{"repoName": "billing-api"})
	require.NoError(t, err)

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].Metadata)
	assert.Equal(t, "billing-api", (*candidates[0].Metadata)["repoName"])
}

func TestPG_UpdateCandidateMetadata_MergesIntoExisting(t *testing.T) {
	s := newPGStore(t)
	existing := map[string]string{"team": "platform"}
	m := pgBaseMigration
	m.Candidates = []api.Candidate{{Id: "billing-api", Kind: "application", Metadata: &existing}}
	require.NoError(t, s.Save(context.Background(), m))

	err := s.UpdateCandidateMetadata(context.Background(), m.Id, "billing-api",
		map[string]string{"repoName": "billing-api"})
	require.NoError(t, err)

	candidates, err := s.GetCandidates(context.Background(), m.Id)
	require.NoError(t, err)
	require.NotNil(t, candidates[0].Metadata)
	assert.Equal(t, "platform", (*candidates[0].Metadata)["team"])
	assert.Equal(t, "billing-api", (*candidates[0].Metadata)["repoName"])
}

func TestPG_UpdateCandidateMetadata_NotFound(t *testing.T) {
	s := newPGStore(t)
	m := pgBaseMigration
	m.Candidates = nil
	require.NoError(t, s.Save(context.Background(), m))

	err := s.UpdateCandidateMetadata(context.Background(), m.Id, "nonexistent",
		map[string]string{"k": "v"})

	assert.Error(t, err)
}
