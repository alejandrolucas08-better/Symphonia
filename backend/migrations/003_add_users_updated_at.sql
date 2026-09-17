-- Older database volumes may have been initialized before users.updated_at
-- existed. CREATE TABLE IF NOT EXISTS in 001 does not change those tables.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
