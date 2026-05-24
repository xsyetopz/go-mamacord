-- migrate:kind=normal
CREATE TABLE IF NOT EXISTS user_settings (
    user_id BIGINT PRIMARY KEY,
    timezone TEXT NOT NULL DEFAULT '',
    dm_channel_id BIGINT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS reminders (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    schedule TEXT NOT NULL,
    kind TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    delivery TEXT NOT NULL,
    guild_id BIGINT,
    channel_id BIGINT,
    enabled BOOLEAN NOT NULL,
    next_run_at BIGINT NOT NULL,
    last_run_at BIGINT,
    failure_count INTEGER NOT NULL DEFAULT 0,
    lease_until BIGINT,
    lease_id TEXT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reminders_user_next
    ON reminders(user_id, next_run_at);

CREATE INDEX IF NOT EXISTS idx_reminders_due
    ON reminders(enabled, next_run_at, lease_until);

CREATE TABLE IF NOT EXISTS checkins (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    mood INTEGER NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_checkins_user_created
    ON checkins(user_id, created_at DESC);
