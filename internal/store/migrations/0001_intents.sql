-- +goose Up
CREATE TABLE intents (
    source TEXT NOT NULL,
    id TEXT NOT NULL,
    subject TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    cancelled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (source, id),
    CONSTRAINT intents_valid_window CHECK (starts_at < ends_at)
);

CREATE INDEX intents_subject_idx ON intents (subject);

CREATE TABLE reconciliation_state (
    subject TEXT PRIMARY KEY,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    next_transition_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX reconciliation_state_due_idx
    ON reconciliation_state (next_transition_at, next_retry_at);

-- +goose Down
DROP TABLE reconciliation_state;
DROP TABLE intents;
