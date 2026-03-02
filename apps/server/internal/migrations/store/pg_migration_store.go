package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// Compile-time check: *PGMigrationStore implements migrations.MigrationStore.
var _ migrations.MigrationStore = (*PGMigrationStore)(nil)

// PGMigrationStore implements MigrationStore using PostgreSQL.
type PGMigrationStore struct {
	pool *pgxpool.Pool
}

// NewPGMigrationStore creates a new PGMigrationStore.
func NewPGMigrationStore(pool *pgxpool.Pool) *PGMigrationStore {
	return &PGMigrationStore{pool: pool}
}

// Save upserts a migration and its candidates within a transaction.
// Candidates in running or completed state are never downgraded.
func (s *PGMigrationStore) Save(ctx context.Context, m api.Migration) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := upsertMigration(ctx, tx, m); err != nil {
		return err
	}

	for _, c := range m.Candidates {
		if c.Status == "" {
			c.Status = api.CandidateStatusNotStarted
		}
		if err := upsertCandidate(ctx, tx, m.Id, c); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Get retrieves a migration by ID with its candidates. Returns nil, nil if not found.
func (s *PGMigrationStore) Get(ctx context.Context, id string) (*api.Migration, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, description, migrator_url, overview, required_inputs, steps, created_at
		 FROM migrations WHERE id = $1`, id)

	m, err := scanMigration(row)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil //nolint:nilnil
	}

	candidates, err := s.queryCandidates(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Candidates = candidates
	return m, nil
}

// List returns all migrations with their candidates.
func (s *PGMigrationStore) List(ctx context.Context) ([]api.Migration, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, description, migrator_url, overview, required_inputs, steps, created_at
		 FROM migrations ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	defer rows.Close()

	var migrations []api.Migration
	var ids []string
	migMap := map[string]*api.Migration{}

	for rows.Next() {
		m, err := scanMigration(rows)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, *m)
		ids = append(ids, m.Id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan migrations: %w", err)
	}
	if len(migrations) == 0 {
		return migrations, nil
	}

	// Build index for candidate assignment.
	for i := range migrations {
		migMap[migrations[i].Id] = &migrations[i]
	}

	candRows, err := s.pool.Query(ctx,
		`SELECT id, migration_id, kind, status, metadata, files, steps
		 FROM candidates WHERE migration_id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	defer candRows.Close()

	for candRows.Next() {
		c, migID, err := scanCandidate(candRows)
		if err != nil {
			return nil, err
		}
		if m, ok := migMap[migID]; ok {
			m.Candidates = append(m.Candidates, c)
		}
	}
	if err := candRows.Err(); err != nil {
		return nil, fmt.Errorf("scan candidates: %w", err)
	}

	return migrations, nil
}

// SetCandidateStatus updates a candidate's status.
func (s *PGMigrationStore) SetCandidateStatus(
	ctx context.Context,
	migrationID, candidateID string,
	status api.CandidateStatus,
) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE candidates SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND migration_id = $3`,
		string(status), candidateID, migrationID)
	if err != nil {
		return fmt.Errorf("set candidate status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("candidate %q not found in migration %q", candidateID, migrationID)
	}
	return nil
}

// SaveCandidates merges the incoming list into the candidates table.
// Candidates already in running or completed state are preserved.
func (s *PGMigrationStore) SaveCandidates(ctx context.Context, migrationID string, incoming []api.Candidate) error {
	// Verify migration exists.
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM migrations WHERE id = $1)`, migrationID).Scan(&exists); err != nil {
		return fmt.Errorf("check migration: %w", err)
	}
	if !exists {
		return fmt.Errorf("migration %q not found", migrationID)
	}

	// Load existing candidates.
	existing, err := s.candidateMap(ctx, migrationID)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	incomingIDs := make(map[string]bool, len(incoming))
	for _, c := range incoming {
		incomingIDs[c.Id] = true

		if ex, ok := existing[c.Id]; ok {
			if ex.Status == api.CandidateStatusRunning || ex.Status == api.CandidateStatusCompleted {
				continue
			}
			// Merge metadata: existing (operator-updated) values win.
			if ex.Metadata != nil {
				if c.Metadata == nil {
					c.Metadata = ex.Metadata
				} else {
					for k, v := range *ex.Metadata {
						(*c.Metadata)[k] = v
					}
				}
			}
		}
		c.Status = api.CandidateStatusNotStarted
		if err := upsertCandidate(ctx, tx, migrationID, c); err != nil {
			return err
		}
	}

	// Re-insert running/completed candidates missing from the incoming list
	// so they stay in the table (they were never removed, but we do a clean upsert).
	for _, ex := range existing {
		if !incomingIDs[ex.Id] &&
			(ex.Status == api.CandidateStatusRunning || ex.Status == api.CandidateStatusCompleted) {
			if err := upsertCandidate(ctx, tx, migrationID, ex); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// GetCandidates returns all candidates for a migration.
func (s *PGMigrationStore) GetCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error) {
	candidates, err := s.queryCandidates(ctx, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get candidates for %q: %w", migrationID, err)
	}
	return candidates, nil
}

// UpdateCandidateMetadata merges key-value pairs into a candidate's metadata using JSONB merge.
func (s *PGMigrationStore) UpdateCandidateMetadata(
	ctx context.Context,
	migrationID, candidateID string,
	metadata map[string]string,
) error {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE candidates
		 SET metadata = COALESCE(metadata, '{}') || $1::jsonb, updated_at = NOW()
		 WHERE id = $2 AND migration_id = $3`,
		metaJSON, candidateID, migrationID)
	if err != nil {
		return fmt.Errorf("update metadata: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("candidate %q not found in migration %q", candidateID, migrationID)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// pgScanner is implemented by both *pgxpool.Row and pgx.Rows.
type pgScanner interface {
	Scan(dest ...any) error
}

func upsertMigration(ctx context.Context, tx pgx.Tx, m api.Migration) error {
	overviewJSON, err := jsonMarshalNullable(m.Overview)
	if err != nil {
		return fmt.Errorf("marshal overview: %w", err)
	}
	requiredInputsJSON, err := jsonMarshalNullable(m.RequiredInputs)
	if err != nil {
		return fmt.Errorf("marshal required_inputs: %w", err)
	}
	stepsJSON, err := json.Marshal(m.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO migrations (id, name, description, migrator_url, overview, required_inputs, steps, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name            = EXCLUDED.name,
			description     = EXCLUDED.description,
			migrator_url    = EXCLUDED.migrator_url,
			overview        = EXCLUDED.overview,
			required_inputs = EXCLUDED.required_inputs,
			steps           = EXCLUDED.steps`,
		m.Id, m.Name, m.Description, m.MigratorUrl,
		overviewJSON, requiredInputsJSON, stepsJSON, m.CreatedAt,
	)
	return err
}

func upsertCandidate(ctx context.Context, tx pgx.Tx, migrationID string, c api.Candidate) error {
	metaJSON, err := jsonMarshalNullable(c.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata for %q: %w", c.Id, err)
	}
	filesJSON, err := jsonMarshalNullable(c.Files)
	if err != nil {
		return fmt.Errorf("marshal files for %q: %w", c.Id, err)
	}
	stepsJSON, err := jsonMarshalNullable(c.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps for %q: %w", c.Id, err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO candidates (id, migration_id, kind, status, metadata, files, steps)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id, migration_id) DO UPDATE SET
			kind       = EXCLUDED.kind,
			status     = CASE
				WHEN candidates.status IN ('running', 'completed') THEN candidates.status
				ELSE EXCLUDED.status
			END,
			metadata   = EXCLUDED.metadata,
			files      = EXCLUDED.files,
			steps      = EXCLUDED.steps,
			updated_at = NOW()`,
		c.Id, migrationID, c.Kind, string(c.Status), metaJSON, filesJSON, stepsJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert candidate %q: %w", c.Id, err)
	}
	return nil
}

func scanMigration(row pgScanner) (*api.Migration, error) {
	var m api.Migration
	var overviewJSON, requiredInputsJSON, stepsJSON []byte

	err := row.Scan(&m.Id, &m.Name, &m.Description, &m.MigratorUrl,
		&overviewJSON, &requiredInputsJSON, &stepsJSON, &m.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil //nolint:nilnil
		}
		return nil, fmt.Errorf("scan migration: %w", err)
	}

	if overviewJSON != nil {
		m.Overview = new([]string)
		if err := json.Unmarshal(overviewJSON, m.Overview); err != nil {
			return nil, fmt.Errorf("unmarshal overview: %w", err)
		}
	}
	if requiredInputsJSON != nil {
		m.RequiredInputs = new([]api.InputDefinition)
		if err := json.Unmarshal(requiredInputsJSON, m.RequiredInputs); err != nil {
			return nil, fmt.Errorf("unmarshal required_inputs: %w", err)
		}
	}
	if stepsJSON != nil {
		if err := json.Unmarshal(stepsJSON, &m.Steps); err != nil {
			return nil, fmt.Errorf("unmarshal steps: %w", err)
		}
	}

	return &m, nil
}

func scanCandidate(row pgScanner) (api.Candidate, string, error) {
	var c api.Candidate
	var migrationID, status string
	var metaJSON, filesJSON, stepsJSON []byte

	err := row.Scan(&c.Id, &migrationID, &c.Kind, &status, &metaJSON, &filesJSON, &stepsJSON)
	if err != nil {
		return c, "", fmt.Errorf("scan candidate: %w", err)
	}
	c.Status = api.CandidateStatus(status)

	if metaJSON != nil {
		c.Metadata = new(map[string]string)
		if err := json.Unmarshal(metaJSON, c.Metadata); err != nil {
			return c, "", fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if filesJSON != nil {
		c.Files = new([]api.FileGroup)
		if err := json.Unmarshal(filesJSON, c.Files); err != nil {
			return c, "", fmt.Errorf("unmarshal files: %w", err)
		}
	}
	if stepsJSON != nil {
		c.Steps = new([]api.StepDefinition)
		if err := json.Unmarshal(stepsJSON, c.Steps); err != nil {
			return c, "", fmt.Errorf("unmarshal steps: %w", err)
		}
	}

	return c, migrationID, nil
}

func (s *PGMigrationStore) queryCandidates(ctx context.Context, migrationID string) ([]api.Candidate, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, migration_id, kind, status, metadata, files, steps
		 FROM candidates WHERE migration_id = $1`, migrationID)
	if err != nil {
		return nil, fmt.Errorf("query candidates: %w", err)
	}
	defer rows.Close()

	var candidates []api.Candidate
	for rows.Next() {
		c, _, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

func (s *PGMigrationStore) candidateMap(ctx context.Context, migrationID string) (map[string]api.Candidate, error) {
	candidates, err := s.queryCandidates(ctx, migrationID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]api.Candidate, len(candidates))
	for _, c := range candidates {
		m[c.Id] = c
	}
	return m, nil
}

func jsonMarshalNullable(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
