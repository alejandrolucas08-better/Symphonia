DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'email_verified_at'
    ) THEN
        ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
        UPDATE users SET email_verified_at = created_at;
    END IF;
END $$;

ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

CREATE TABLE IF NOT EXISTS auth_tokens (
    token_hash BYTEA PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose VARCHAR(20) NOT NULL CHECK (purpose IN ('verify_email', 'reset_password', 'oauth_ticket')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_purpose ON auth_tokens (user_id, purpose);

CREATE TABLE IF NOT EXISTS oauth_identities (
    provider VARCHAR(20) NOT NULL CHECK (provider = 'google'),
    subject VARCHAR(255) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (provider, subject),
    UNIQUE (provider, user_id)
);
