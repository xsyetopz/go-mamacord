-- migrate:kind=normal
CREATE TABLE IF NOT EXISTS users (
    user_id BIGINT PRIMARY KEY,
    created_at BIGINT NOT NULL,
    is_bot BOOLEAN NOT NULL,
    is_system BOOLEAN NOT NULL,
    first_seen_at BIGINT NOT NULL,
    last_seen_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_last_seen
    ON users(last_seen_at DESC);

CREATE TABLE IF NOT EXISTS guilds (
    guild_id BIGINT PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    joined_at BIGINT NOT NULL,
    left_at BIGINT,
    name TEXT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_guilds_owner
    ON guilds(owner_id);

CREATE TABLE IF NOT EXISTS guild_members (
    guild_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    joined_at BIGINT NOT NULL,
    left_at BIGINT,
    PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_guild_members_user
    ON guild_members(user_id);
