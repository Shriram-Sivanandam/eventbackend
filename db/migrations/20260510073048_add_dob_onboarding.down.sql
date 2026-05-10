ALTER TABLE users
DROP COLUMN IF EXISTS onboarding_complete,
DROP COLUMN IF EXISTS date_of_birth;