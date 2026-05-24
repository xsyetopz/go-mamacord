-- migrate:kind=normal
CREATE TABLE IF NOT EXISTS trusted_signers (
    key_id TEXT PRIMARY KEY,
    public_key_b64 TEXT NOT NULL,
    added_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS restrictions (
    target_type TEXT NOT NULL,
    target_id BIGINT NOT NULL,
    reason TEXT NOT NULL,
    created_by BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (target_type, target_id)
);

CREATE TABLE IF NOT EXISTS warnings (
    id TEXT PRIMARY KEY,
    guild_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    moderator_id BIGINT NOT NULL,
    reason TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_warnings_guild_user_created
    ON warnings(guild_id, user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS plugin_kv (
    guild_id BIGINT NOT NULL,
    plugin_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value_json TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (guild_id, plugin_id, key)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    guild_id BIGINT,
    actor_id BIGINT,
    action TEXT NOT NULL,
    target_type TEXT,
    target_id BIGINT,
    created_at BIGINT NOT NULL,
    meta_json TEXT NOT NULL DEFAULT '{}'
);
