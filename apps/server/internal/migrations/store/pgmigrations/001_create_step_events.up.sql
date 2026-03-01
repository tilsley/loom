CREATE TABLE step_events (
    id           BIGSERIAL   PRIMARY KEY,
    migration_id TEXT        NOT NULL,
    candidate_id TEXT        NOT NULL,
    step_name    TEXT,
    event_type   TEXT        NOT NULL,
    status       TEXT,
    duration_ms  INTEGER,
    metadata     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_step_events_migration ON step_events (migration_id);
CREATE INDEX idx_step_events_migration_candidate ON step_events (migration_id, candidate_id);
CREATE INDEX idx_step_events_event_type ON step_events (event_type);
CREATE INDEX idx_step_events_created_at ON step_events (created_at);
CREATE INDEX idx_step_events_step_name ON step_events (step_name);
