-- migrate:kind=normal
CREATE TABLE IF NOT EXISTS plugin_sources (
  source_id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  git_url TEXT NOT NULL,
  git_ref TEXT,
  git_subdir TEXT,
  token_env_var TEXT,
  enabled BOOLEAN NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS plugin_source_sync (
  source_id TEXT PRIMARY KEY,
  last_synced_at BIGINT,
  last_revision TEXT,
  last_error TEXT
);

CREATE TABLE IF NOT EXISTS plugin_installs (
  plugin_id TEXT PRIMARY KEY,
  install_kind TEXT NOT NULL,
  source_id TEXT,
  git_url TEXT NOT NULL,
  git_ref TEXT,
  git_revision TEXT NOT NULL,
  source_path TEXT NOT NULL,
  installed_at BIGINT NOT NULL,
  installed_by BIGINT,
  installed_hash_b64 TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trusted_vendors (
  vendor_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  website_url TEXT,
  support_url TEXT,
  added_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS trusted_vendor_keys (
  vendor_id TEXT NOT NULL,
  key_id TEXT NOT NULL,
  PRIMARY KEY (vendor_id, key_id)
);
