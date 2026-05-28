ALTER TABLE users
  DROP COLUMN IF EXISTS fcm_token,
  DROP COLUMN IF EXISTS fcm_updated_at;