CREATE TABLE IF NOT EXISTS files (
  id TEXT PRIMARY KEY,
  owner_sub TEXT NOT NULL,
  object_key TEXT NOT NULL UNIQUE,
  content_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  purpose TEXT NOT NULL DEFAULT 'drive',
  status TEXT NOT NULL DEFAULT 'pending',
  variants JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS files_owner_created ON files (owner_sub, created_at DESC);

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  file_id TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_quota (
  owner_sub TEXT PRIMARY KEY,
  used_bytes BIGINT NOT NULL DEFAULT 0
);
