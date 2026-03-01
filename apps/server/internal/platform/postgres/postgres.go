package postgres

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates a pgx connection pool and runs any pending SQL migrations.
// The connString must be a valid PostgreSQL URL (e.g. postgres://user:pass@host/db).
// Migrations are read from the provided embed.FS.
func New(ctx context.Context, connString string, migrations fs.FS) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgx ping: %w", err)
	}

	if err := runMigrations(connString, migrations); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return pool, nil
}

func runMigrations(connString string, migrations fs.FS) error {
	src, err := iofs.New(migrations, ".")
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}

	// golang-migrate's pgx/v5 driver expects a "pgx5://" scheme.
	// Convert "postgres://" → "pgx5://" so the driver is selected automatically.
	dbURL := pgx5URL(connString)
	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// pgx5URL replaces a "postgres://" or "postgresql://" scheme with "pgx5://"
// so that golang-migrate selects its pgx/v5 database driver.
func pgx5URL(connString string) string {
	const (
		postgres   = "postgres://"
		postgresql = "postgresql://"
		pgx5       = "pgx5://"
	)
	if len(connString) >= len(postgresql) && connString[:len(postgresql)] == postgresql {
		return pgx5 + connString[len(postgresql):]
	}
	if len(connString) >= len(postgres) && connString[:len(postgres)] == postgres {
		return pgx5 + connString[len(postgres):]
	}
	return connString
}
