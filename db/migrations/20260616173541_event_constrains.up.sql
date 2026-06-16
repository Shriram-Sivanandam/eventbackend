-- Drop the existing FK constraint
ALTER TABLE events DROP CONSTRAINT events_host_user_id_fkey;

-- Re-add it with ON DELETE SET NULL
-- host_user_id must also be nullable for this to work
ALTER TABLE events ALTER COLUMN host_user_id DROP NOT NULL;

ALTER TABLE events
  ADD CONSTRAINT events_host_user_id_fkey
  FOREIGN KEY (host_user_id)
  REFERENCES users(id)
  ON DELETE SET NULL;