CREATE TABLE IF NOT EXISTS folders (
  id TEXT PRIMARY KEY,
  owner_sub TEXT NOT NULL,
  parent_id TEXT REFERENCES folders(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS folders_owner_parent_name
  ON folders (owner_sub, COALESCE(parent_id, ''), name);

CREATE INDEX IF NOT EXISTS folders_owner_parent ON folders (owner_sub, parent_id);

ALTER TABLE files ADD COLUMN IF NOT EXISTS folder_id TEXT REFERENCES folders(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS files_owner_folder ON files (owner_sub, folder_id);
