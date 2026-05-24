-- migrate:kind=normal
CREATE TABLE IF NOT EXISTS module_states (
    module_id TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL,
    updated_at BIGINT NOT NULL,
    updated_by BIGINT
);
