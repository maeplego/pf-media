ALTER TABLE files ADD COLUMN IF NOT EXISTS org_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS files_org_owner_created ON files (org_id, owner_sub, created_at DESC);

ALTER TABLE folders ADD COLUMN IF NOT EXISTS org_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS folders_org_owner ON folders (org_id, owner_sub);

ALTER TABLE share_links ADD COLUMN IF NOT EXISTS org_id TEXT NOT NULL DEFAULT '';
