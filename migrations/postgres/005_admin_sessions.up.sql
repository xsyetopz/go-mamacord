-- migrate:kind=normal
CREATE TABLE IF NOT EXISTS admin_sessions (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  username TEXT NOT NULL,
  name TEXT NOT NULL,
  avatar_url TEXT NOT NULL,
  csrf_token TEXT NOT NULL,
  access_token TEXT NOT NULL,
  is_owner BOOLEAN NOT NULL,
  expires_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at ON admin_sessions(expires_at);
