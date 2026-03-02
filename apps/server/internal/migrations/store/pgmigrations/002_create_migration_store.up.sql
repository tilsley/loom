CREATE TABLE migrations (
    id              TEXT        PRIMARY KEY,
    name            TEXT        NOT NULL DEFAULT '',
    description     TEXT        NOT NULL DEFAULT '',
    migrator_url    TEXT        NOT NULL DEFAULT '',
    overview        JSONB,
    required_inputs JSONB,
    steps           JSONB       NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE candidates (
    id           TEXT        NOT NULL,
    migration_id TEXT        NOT NULL REFERENCES migrations(id),
    kind         TEXT        NOT NULL DEFAULT '',
    status       TEXT        NOT NULL DEFAULT 'not_started',
    metadata     JSONB,
    files        JSONB,
    steps        JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, migration_id)
);

CREATE INDEX idx_candidates_migration ON candidates (migration_id);
