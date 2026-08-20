-- migrate:kind=normal
ALTER TABLE plugin_kv
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);

INSERT INTO plugin_kv (guild_id, plugin_id, key, value_json, updated_at)
SELECT guild_id, plugin_id, 'guild_config', value_json, updated_at
FROM plugin_kv
WHERE key = '__guild_config'
ON CONFLICT (guild_id, plugin_id, key) DO NOTHING;

DELETE FROM plugin_kv
WHERE key = '__guild_config';
